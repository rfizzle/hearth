package orchestrate

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/orchestrate/park"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// ErrBusy refuses a message to a conversation whose bot is still answering
// the last one.
var ErrBusy = errors.New("the bot is still working on the last message")

// ErrWaiting refuses a message to a conversation whose bot is waiting for
// the owner to decide or answer.
// See docs/model.md#8-approvals.
var ErrWaiting = errors.New("answer what the bot asked first")

// ErrPaused refuses a message to a bot, or to any bot while the org is
// paused, whose month's budget is spent.
// See docs/model.md#usage.
var ErrPaused = errors.New("this month's budget is spent; resume in Usage or raise the budget")

// quitReason is the failure a run left working by an earlier launch ends
// with.
const quitReason = "hearth was quit while this was running"

// Deps is what the service is handed.
type Deps struct {
	Store *store.DB
	// Streams opens the model the bot's tier resolves to.
	Streams func(org.Tier) (agent.Stream, error)
	// Models names the provider and model the tier resolves to, for the
	// usage each request is priced by; nil leaves them unnamed, and the
	// requests unpriced.
	Models func(org.Tier) agent.Model
	// Meter records each request's usage and holds the budgets; nil meters
	// nothing and pauses nothing. Its OnPause should be StopForBudget.
	Meter *meter.Meter
	// Scrub is applied to every message the model is sent and every tool
	// result; nil scrubs nothing.
	Scrub agent.Scrub
	// Fetcher reads web pages; nil leaves web.fetch out.
	Fetcher *web.Fetcher
	// Searcher is asked once per run for the configured search service, so
	// a key set after launch counts from the next message; nil, or false,
	// leaves web.search out.
	Searcher func() (web.Searcher, bool)
	Bus      *Bus
	// Reserved are folders no tool may touch, whatever a bot is granted.
	Reserved []string
	// Resume is how a run that continues a parked one is handed the round
	// it parked in.
	Resume park.Resume
	// Caps are the limits every run ends at; zero takes the defaults.
	Caps org.Caps
	// Log is where a run that panicked is reported, with its stack; nil is
	// the standard logger.
	Log *log.Logger
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// Service starts and stops the runs that answer the owner's messages, one
// run per conversation at a time.
type Service struct {
	d Deps

	mu      sync.Mutex
	working map[string]string             // conversation id -> its run's id
	cancels map[string]context.CancelFunc // run id -> what stops it
	spent   map[string]string             // run id -> why the budget stopped it
	held    map[string]*held              // item id -> the answer waiting out its grace

	grace time.Duration      // answerGrace, shorter in a test
	extra []tools.Definition // more tools, for a test
	fault func() error       // fails recording a decision, for a test
}

// Open is a service over d. Runs an earlier launch left queued or working
// are ended as failed first: nothing resumes them, and left alone their bot
// would show as working forever. Messages it left streaming are closed, or
// deleted when they hold nothing, so no bubble stays open.
// See docs/model.md#9-failure-handling.
func Open(ctx context.Context, d Deps) (*Service, error) {
	if d.Store == nil || d.Streams == nil || d.Bus == nil {
		return nil, errors.New("orchestrate: a store, a stream source and a bus are required")
	}
	if d.Caps == (org.Caps{}) {
		d.Caps = org.DefaultCaps()
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	if _, err := d.Store.FailWorkingRuns(ctx, quitReason); err != nil {
		return nil, fmt.Errorf("orchestrate: %w", err)
	}
	if _, _, err := d.Store.CloseStreamingMessages(ctx); err != nil {
		return nil, fmt.Errorf("orchestrate: %w", err)
	}
	if d.Log == nil {
		d.Log = log.Default()
	}
	return &Service{d: d, working: map[string]string{}, cancels: map[string]context.CancelFunc{}, spent: map[string]string{},
		held: map[string]*held{}, grace: answerGrace}, nil
}

// Send stores text as the owner's message in the DM convID and starts the
// run that answers it, returning both as stored. The run goes on after Send
// returns; its progress is published on the bus. Send refuses with ErrBusy
// while an earlier run in the conversation has not ended. If the run cannot
// be stored the owner's message is removed again, so no message is left that
// nothing answers. The Inbox items earlier failures in the conversation
// left are closed: the new run answers the owner instead.
func (s *Service) Send(ctx context.Context, convID, text string) (org.Run, org.Message, error) {
	bot, err := s.dmBot(ctx, convID)
	if err != nil {
		return org.Run{}, org.Message{}, err
	}
	if err := s.paused(ctx, bot); err != nil {
		return org.Run{}, org.Message{}, err
	}
	if err := s.reserveIdle(ctx, convID); err != nil {
		return org.Run{}, org.Message{}, err
	}
	m, err := s.d.Store.AppendMessage(ctx, org.Message{ConversationID: convID, Kind: org.MessageText, Content: text})
	if err != nil {
		s.release(convID, "")
		return org.Run{}, org.Message{}, err
	}
	s.d.Bus.Publish(MessageUpserted{Message: m})
	r, runCtx, err := s.start(ctx, bot, m)
	if err != nil {
		s.unsend(ctx, m)
		s.release(convID, "")
		return org.Run{}, org.Message{}, err
	}
	s.superseded(context.WithoutCancel(ctx), convID, "Closed: a new message was sent")
	go s.run(runCtx, bot, r, ask{msg: m})
	return r, m, nil
}

// Retry answers the owner's last message in the DM convID again, as a new
// run from scratch; the runs before it are left as they are, and the Inbox
// items their failures left are closed. It refuses with ErrBusy while a run
// in the conversation is still working.
// See docs/model.md#9-failure-handling.
func (s *Service) Retry(ctx context.Context, convID string) (org.Run, error) {
	bot, err := s.dmBot(ctx, convID)
	if err != nil {
		return org.Run{}, err
	}
	if err := s.paused(ctx, bot); err != nil {
		return org.Run{}, err
	}
	if err := s.reserveIdle(ctx, convID); err != nil {
		return org.Run{}, err
	}
	prompt, err := s.lastOwnerMessage(ctx, convID)
	if err != nil {
		s.release(convID, "")
		return org.Run{}, err
	}
	r, runCtx, err := s.start(ctx, bot, prompt)
	if err != nil {
		s.release(convID, "")
		return org.Run{}, err
	}
	s.superseded(context.WithoutCancel(ctx), convID, "Retried")
	go s.run(runCtx, bot, r, ask{msg: prompt})
	return r, nil
}

// paused is ErrPaused, naming the bot, when its budget or the org's is spent
// this month and not resumed.
func (s *Service) paused(ctx context.Context, bot org.Bot) error {
	if s.d.Meter == nil {
		return nil
	}
	paused, err := s.d.Meter.Paused(ctx, bot.ID)
	if err != nil {
		return err
	}
	if paused {
		return fmt.Errorf("%s is paused: %w", bot.Name, ErrPaused)
	}
	return nil
}

// StopForBudget stops the run runID, which a request of its found the
// budget spent: the run ends failed with reason, keeping what it had
// written. It is the meter's OnPause.
func (s *Service) StopForBudget(runID, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.cancels[runID]; ok {
		s.spent[runID] = reason
		cancel()
	}
}

// reserve claims the conversation for a run about to start, or is ErrBusy.
func (s *Service) reserve(convID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, busy := s.working[convID]; busy {
		return ErrBusy
	}
	s.working[convID] = ""
	return nil
}

// reserveIdle claims the conversation for a run the owner asked for, or is
// ErrBusy, or ErrWaiting while the bot waits on them: the open item is
// answered first, and the run it resumes is the next to start.
// See docs/model.md#8-approvals.
func (s *Service) reserveIdle(ctx context.Context, convID string) error {
	if err := s.reserve(convID); err != nil {
		return err
	}
	it, open, err := s.d.Store.OpenItemFor(ctx, convID)
	if err == nil && open {
		err = s.stale(ctx, it)
	}
	if err != nil {
		s.release(convID, "")
	}
	return err
}

// stale closes it when the run it was opened for no longer waits on it, as
// when the core stopped between storing the item and the run, and is
// ErrWaiting when the run still waits: an item nothing can settle must not
// hold the conversation for good.
func (s *Service) stale(ctx context.Context, it inbox.Item) error {
	r, err := s.d.Store.Run(ctx, it.RunID)
	if err == nil && r.State == org.RunWaitingOwner {
		return ErrWaiting
	}
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if err := s.d.Store.CloseInboxItem(ctx, it.ID, 0, "", "Closed: the run had ended"); err != nil {
		return err
	}
	_, err = s.closed(ctx, it.ID)
	return err
}

// start stores the queued run that answers prompt and registers what stops
// it.
func (s *Service) start(ctx context.Context, bot org.Bot, prompt org.Message) (org.Run, context.Context, error) {
	r, err := s.d.Store.CreateRun(ctx, org.Run{ConversationID: prompt.ConversationID, BotID: bot.ID, State: org.RunQueued})
	if err != nil {
		return org.Run{}, nil, err
	}
	// The run outlives the request that started it; only Cancel stops it.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.working[r.ConversationID] = r.ID
	s.cancels[r.ID] = cancel
	s.mu.Unlock()
	return r, runCtx, nil
}

// unsend removes the owner's message m again.
func (s *Service) unsend(ctx context.Context, m org.Message) {
	if err := s.d.Store.DeleteMessage(context.WithoutCancel(ctx), m.ID); err != nil {
		s.d.Log.Printf("orchestrate: could not remove message %s after its run failed to start: %v", m.ID, err)
		return
	}
	s.d.Bus.Publish(MessageDeleted{ConversationID: m.ConversationID, MessageID: m.ID})
}

// lastOwnerMessage is the newest text the owner wrote in convID.
func (s *Service) lastOwnerMessage(ctx context.Context, convID string) (org.Message, error) {
	var before int64
	for {
		msgs, more, err := s.d.Store.Messages(ctx, convID, before, 200)
		if err != nil {
			return org.Message{}, err
		}
		for _, m := range slices.Backward(msgs) {
			if m.Kind == org.MessageText && m.AuthorBotID == "" {
				return m, nil
			}
		}
		if !more {
			return org.Message{}, fmt.Errorf("conversation %s has no message of yours to answer: %w", convID, store.ErrNotFound)
		}
		before = msgs[0].Seq
	}
}

// release frees the conversation for the next message.
func (s *Service) release(convID, runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cancel, ok := s.cancels[runID]; ok {
		cancel()
		delete(s.cancels, runID)
	}
	delete(s.working, convID)
}

// Cancel stops the run runID, keeping what it had written so far, and
// reports whether a run was working or waiting to stop. A run waiting for
// the owner ends cancelled, and the item it waited on closes.
func (s *Service) Cancel(runID string) bool {
	s.mu.Lock()
	cancel, ok := s.cancels[runID]
	if ok {
		cancel()
	}
	s.mu.Unlock()
	return ok || s.cancelWaiting(context.Background(), runID)
}

// dmBot is the bot whose DM convID is.
func (s *Service) dmBot(ctx context.Context, convID string) (org.Bot, error) {
	bots, err := s.d.Store.Bots(ctx)
	if err != nil {
		return org.Bot{}, err
	}
	for _, b := range bots {
		if b.DMConversationID == convID {
			return b, nil
		}
	}
	return org.Bot{}, fmt.Errorf("conversation %s: %w", convID, store.ErrNotFound)
}
