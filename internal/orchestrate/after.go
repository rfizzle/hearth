package orchestrate

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/orchestrate/record"
	"github.com/rfizzle/hearth/internal/org"
)

// after is the work a run that ended done leaves: tidying the bot's notes
// when they have reached the cap, then folding what the run's window left
// out into the conversation's summary. Both ask the Fast tier's model, and
// their usage counts against the run k recorded. A failure is logged and
// tried again after the next run.
// See docs/model.md#2-a-run and docs/model.md#7-memory-writes.
func (s *Service) after(ctx context.Context, k *record.Sink, bot org.Bot, built assembly.Output) {
	r := k.Run()
	side := func() (assembly.Side, error) {
		stream, err := s.d.Streams(org.Bot{Tier: org.TierFast})
		if err != nil {
			return assembly.Side{}, err
		}
		side := assembly.Side{Stream: stream, Scrub: s.d.Scrub, Usage: k.Usage}
		if s.d.Models != nil {
			side.Model = s.d.Models(org.Bot{Tier: org.TierFast})
		}
		return side, nil
	}
	s.tidy(ctx, side, bot, r)
	if built.Behind {
		s.summarise(ctx, side, r.ConversationID, built.Until)
	}
}

// tidy consolidates bot's notes when they have reached the cap, says so in
// the thread of the run r and publishes each note it removed and added. It
// holds the conversation while it works, since a note the bot writes
// between the read and the replace would be lost: a bot has one
// conversation, so the bot is not running. If a new run holds it already,
// the notes are tidied after that one.
func (s *Service) tidy(ctx context.Context, side func() (assembly.Side, error), bot org.Bot, r org.Run) {
	was, err := s.d.Store.Notes(ctx, bot.ID)
	if err != nil || !assembly.TidyDue(len(was)) {
		return
	}
	if s.reserve(r.ConversationID) != nil {
		return
	}
	defer s.release(r.ConversationID, "")
	sd, err := side()
	if err != nil {
		s.d.Log.Printf("orchestrate: %s's notes were not tidied: %v", bot.Name, err)
		return
	}
	before, after, err := assembly.Consolidate(ctx, s.d.Store, sd, bot.ID)
	if err != nil {
		s.d.Log.Printf("orchestrate: %s's notes were not tidied: %v", bot.Name, err)
		return
	}
	if now, err := s.d.Store.Notes(ctx, bot.ID); err == nil {
		gone, added := assembly.Changed(was, now)
		for _, n := range gone {
			s.d.Bus.Publish(NoteChanged{BotID: bot.ID, Note: n, Deleted: true})
		}
		for _, n := range added {
			s.d.Bus.Publish(NoteChanged{BotID: bot.ID, Note: n})
		}
	}
	m, err := s.d.Store.AppendMessage(ctx, org.Message{
		ConversationID: r.ConversationID, AuthorBotID: bot.ID, RunID: r.ID, Kind: org.MessageSystem,
		Content: fmt.Sprintf("%s tidied their notes: %d became %d", bot.Name, before, after),
	})
	if err != nil {
		s.d.Log.Printf("orchestrate: run %s: %v", r.ID, err)
		return
	}
	s.d.Bus.Publish(MessageUpserted{Message: m})
}

// summarise folds the conversation convID's text before until that its
// summary does not cover into it and publishes the conversation, one
// summary at a time per conversation: one already being written is left to
// finish, and what it does not cover is folded in after the next run.
func (s *Service) summarise(ctx context.Context, side func() (assembly.Side, error), convID string, until int64) {
	s.mu.Lock()
	if s.summing[convID] {
		s.mu.Unlock()
		return
	}
	s.summing[convID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.summing, convID)
		s.mu.Unlock()
	}()
	sd, err := side()
	if err == nil {
		err = assembly.Summarise(ctx, s.d.Store, sd, convID, until)
	}
	if err != nil {
		s.d.Log.Printf("orchestrate: conversation %s was not summarised: %v", convID, err)
		return
	}
	c, err := s.d.Store.Conversation(ctx, convID)
	if err != nil {
		s.d.Log.Printf("orchestrate: conversation %s: %v", convID, err)
		return
	}
	s.d.Bus.Publish(ConversationChanged{Conversation: c})
}
