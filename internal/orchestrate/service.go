package orchestrate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// ErrBusy refuses a message to a conversation whose bot is still answering
// the last one.
var ErrBusy = errors.New("the bot is still working on the last message")

// quitReason is the failure a run left working by an earlier launch ends
// with.
const quitReason = "hearth was quit while this was running"

// Deps is what the service is handed.
type Deps struct {
	Store *store.DB
	// Streams opens the model the bot's tier resolves to.
	Streams func(org.Tier) (agent.Stream, error)
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
	// Caps are the limits every run ends at; zero takes the defaults.
	Caps org.Caps
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
}

// Open is a service over d. Runs an earlier launch left queued or working
// are ended as failed first: nothing resumes them, and left alone their bot
// would show as working forever.
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
	return &Service{d: d, working: map[string]string{}, cancels: map[string]context.CancelFunc{}}, nil
}

// Send stores text as the owner's message in the DM convID and starts the
// run that answers it, returning both as stored. The run goes on after Send
// returns; its progress is published on the bus. Send refuses with ErrBusy
// while an earlier run in the conversation has not ended.
func (s *Service) Send(ctx context.Context, convID, text string) (org.Run, org.Message, error) {
	bot, err := s.dmBot(ctx, convID)
	if err != nil {
		return org.Run{}, org.Message{}, err
	}
	s.mu.Lock()
	if _, busy := s.working[convID]; busy {
		s.mu.Unlock()
		return org.Run{}, org.Message{}, ErrBusy
	}
	s.working[convID] = ""
	s.mu.Unlock()

	r, m, runCtx, err := s.start(ctx, bot, convID, text)
	if err != nil {
		s.release(convID, "")
		return org.Run{}, org.Message{}, err
	}
	go s.run(runCtx, bot, r, m)
	return r, m, nil
}

// start stores the owner's message and the queued run, and registers what
// stops the run.
func (s *Service) start(ctx context.Context, bot org.Bot, convID, text string) (org.Run, org.Message, context.Context, error) {
	m, err := s.d.Store.AppendMessage(ctx, org.Message{ConversationID: convID, Kind: org.MessageText, Content: text})
	if err != nil {
		return org.Run{}, org.Message{}, nil, err
	}
	s.d.Bus.Publish(MessageUpserted{Message: m})
	r, err := s.d.Store.CreateRun(ctx, org.Run{ConversationID: convID, BotID: bot.ID, State: org.RunQueued})
	if err != nil {
		return org.Run{}, org.Message{}, nil, err
	}
	// The run outlives the request that started it; only Cancel stops it.
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.working[convID] = r.ID
	s.cancels[r.ID] = cancel
	s.mu.Unlock()
	return r, m, runCtx, nil
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
// reports whether a run was working to stop.
func (s *Service) Cancel(runID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cancel, ok := s.cancels[runID]
	if ok {
		cancel()
	}
	return ok
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
