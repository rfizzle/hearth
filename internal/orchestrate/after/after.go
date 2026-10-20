package after

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
)

// Input is what one call of Run is handed. It is read afresh for each run, so
// a scrub or a log the service changes counts from the next one.
type Input struct {
	Store *store.DB
	Bus   *bus.Bus
	Log   *log.Logger
	// Streams opens the model of the bot it is asked for; Run asks for the
	// Fast tier's. Models names its provider and model, and may be nil.
	Streams func(org.Bot) (agent.Stream, error)
	Models  func(org.Bot) agent.Model
	Scrub   agent.Scrub
	// Run is the run that ended done, and Usage reports a request to the
	// meter against it.
	Run   org.Run
	Usage func(provider, model string, u agent.Usage)
	// Reserve claims the conversation, and is an error while a run holds it;
	// Release frees it again.
	Reserve func(convID string) error
	Release func(convID string)
}

// Worker does the work after runs. The zero value is ready to use and must
// not be copied once used.
type Worker struct {
	mu      sync.Mutex
	summing map[string]bool // conversation id -> its summary is being written
}

// Run is the work a run that ended done leaves: tidying the bot's notes
// when they have reached the cap, then folding what the run's window left
// out into the conversation's summary. Both ask the Fast tier's model, and
// their usage counts against the run in. A failure is logged and tried
// again after the next run.
// See docs/model.md#2-a-run and docs/model.md#7-memory-writes.
func (w *Worker) Run(ctx context.Context, in Input, bot org.Bot, built assembly.Output) {
	side := func() (assembly.Side, error) {
		stream, err := in.Streams(org.Bot{Tier: org.TierFast})
		if err != nil {
			return assembly.Side{}, err
		}
		side := assembly.Side{Stream: stream, Scrub: in.Scrub, Usage: in.Usage}
		if in.Models != nil {
			side.Model = in.Models(org.Bot{Tier: org.TierFast})
		}
		return side, nil
	}
	tidy(ctx, in, side, bot)
	if built.Behind {
		w.summarise(ctx, in, side, built.Until)
	}
}

// tidy consolidates bot's notes when they have reached the cap, says so in
// the thread of the run and publishes each note it removed and added. It
// holds the conversation while it works, since a note the bot writes
// between the read and the replace would be lost: a bot has one
// conversation, so the bot is not running. If a new run holds it already,
// the notes are tidied after that one.
func tidy(ctx context.Context, in Input, side func() (assembly.Side, error), bot org.Bot) {
	r := in.Run
	was, err := in.Store.Notes(ctx, bot.ID)
	if err != nil || !assembly.TidyDue(len(was)) {
		return
	}
	if in.Reserve(r.ConversationID) != nil {
		return
	}
	defer in.Release(r.ConversationID)
	sd, err := side()
	if err != nil {
		in.Log.Printf("orchestrate: %s's notes were not tidied: %v", bot.Name, err)
		return
	}
	before, after, err := assembly.Consolidate(ctx, in.Store, sd, bot.ID)
	if err != nil {
		in.Log.Printf("orchestrate: %s's notes were not tidied: %v", bot.Name, err)
		return
	}
	if now, err := in.Store.Notes(ctx, bot.ID); err == nil {
		gone, added := assembly.Changed(was, now)
		for _, n := range gone {
			in.Bus.Publish(bus.NoteChanged{BotID: bot.ID, Note: n, Deleted: true})
		}
		for _, n := range added {
			in.Bus.Publish(bus.NoteChanged{BotID: bot.ID, Note: n})
		}
	}
	m, err := in.Store.AppendMessage(ctx, org.Message{
		ConversationID: r.ConversationID, AuthorBotID: bot.ID, RunID: r.ID, Kind: org.MessageSystem,
		Content: fmt.Sprintf("%s tidied their notes: %d became %d", bot.Name, before, after),
	})
	if err != nil {
		in.Log.Printf("orchestrate: run %s: %v", r.ID, err)
		return
	}
	in.Bus.Publish(bus.MessageUpserted{Message: m})
}

// summarise folds the run's conversation's text before until that its
// summary does not cover into it and publishes the conversation, one
// summary at a time per conversation: one already being written is left to
// finish, and what it does not cover is folded in after the next run.
func (w *Worker) summarise(ctx context.Context, in Input, side func() (assembly.Side, error), until int64) {
	convID := in.Run.ConversationID
	w.mu.Lock()
	if w.summing[convID] {
		w.mu.Unlock()
		return
	}
	if w.summing == nil {
		w.summing = map[string]bool{}
	}
	w.summing[convID] = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.summing, convID)
		w.mu.Unlock()
	}()
	sd, err := side()
	if err == nil {
		err = assembly.Summarise(ctx, in.Store, sd, convID, until)
	}
	if err != nil {
		in.Log.Printf("orchestrate: conversation %s was not summarised: %v", convID, err)
		return
	}
	c, err := in.Store.Conversation(ctx, convID)
	if err != nil {
		in.Log.Printf("orchestrate: conversation %s: %v", convID, err)
		return
	}
	in.Bus.Publish(bus.ConversationChanged{Conversation: c})
}
