package orchestrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rfizzle/hearth/internal/orchestrate/park"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/widget"
)

// answerGrace is how long an answer stays open to a change before the run
// resumes on it: a mis-click can be taken back for that long.
const answerGrace = 2 * time.Second

// held is an answer waiting out its grace.
type held struct {
	reply park.Reply
	timer *time.Timer
}

// Answer answers the question itemID with the owner's words, their answer
// to its control, or both, and returns the item with the answer and the run
// still waiting on it. An answer the control does not allow wraps
// widget.ErrInvalid. The answer shows on the question in the thread at
// once; the run carries on after a short grace, and an answer given inside
// it replaces the one before. Then the owner's words and the bot's fixed
// words for them are recorded, the item closes and the run that carries on
// starts.
// See docs/model.md#owner-interaction.
func (s *Service) Answer(ctx context.Context, itemID, text string, value *widget.Answer) (inbox.Item, org.Run, error) {
	p, err := s.load(ctx, itemID, inbox.Question)
	if err != nil {
		return inbox.Item{}, org.Run{}, err
	}
	r, err := park.Answer(p.item.Widget, text, value)
	if err != nil && !errors.Is(err, widget.ErrInvalid) {
		err = fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	if err != nil {
		return inbox.Item{}, org.Run{}, err
	}
	if err := s.showAnswer(context.WithoutCancel(ctx), p.item, r.Value); err != nil {
		return inbox.Item{}, org.Run{}, err
	}
	s.hold(itemID, r, true)
	p.item.Answer, p.item.WidgetAnswer = r.Owner, r.Value
	return p.item, p.run, nil
}

// showAnswer puts the answer to the control on the question and its item,
// or takes an earlier one off when the owner answered again in words.
func (s *Service) showAnswer(ctx context.Context, it inbox.Item, value string) error {
	if it.Widget == "" {
		return nil
	}
	m, err := s.d.Store.Message(ctx, it.MessageID)
	if err != nil || m.WidgetAnswer == value {
		return err
	}
	m.WidgetAnswer = value
	if err := s.d.Store.AnswerInboxItem(ctx, it.ID, value); err != nil {
		return err
	}
	if err := s.d.Store.UpdateMessage(ctx, m); err != nil {
		return err
	}
	s.d.Bus.Publish(MessageUpserted{Message: m})
	return nil
}

// hold keeps r as the answer to itemID until the grace ends. An answer
// already held is replaced, and the grace starts again, only when replace
// is set: a retry must not undo an answer given since.
func (s *Service) hold(itemID string, r park.Reply, replace bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.held[itemID]; ok {
		if replace {
			h.reply = r
			h.timer.Reset(s.grace)
		}
		return
	}
	s.held[itemID] = &held{reply: r, timer: time.AfterFunc(s.grace, func() { s.answered(itemID) })}
}

// answered settles itemID with the answer held for it once the grace ends.
// An item settled or cancelled meanwhile is left as it is.
func (s *Service) answered(itemID string) {
	s.mu.Lock()
	h := s.held[itemID]
	delete(s.held, itemID)
	s.mu.Unlock()
	if h == nil {
		return
	}
	ctx := context.Background()
	p, err := s.claim(ctx, itemID, inbox.Question)
	if err == nil {
		owner := org.Message{Kind: org.MessageText, Content: h.reply.Owner}
		result := func() outcome { return outcome{text: h.reply.Result} }
		_, _, err = s.settle(ctx, p, owner, result, 0, h.reply.Owner, "Answered")
	}
	switch {
	case errors.Is(err, ErrBusy):
		// A message being refused holds the conversation for a moment.
		s.hold(itemID, h.reply, false)
	case err != nil && !errors.Is(err, ErrClosed):
		s.d.Log.Printf("orchestrate: the answer to %s could not be recorded: %v", itemID, err)
	}
}
