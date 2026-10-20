package rpc

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
	"github.com/rfizzle/hearth/internal/skill"
	"github.com/rfizzle/hearth/internal/store"
)

// step answers one request of the scripted model.
type step func(ctx context.Context) <-chan llm.StreamEvent

// answer streams the words as tokens and ends.
func answer(words ...string) step {
	return func(context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent, len(words)+1)
		for _, w := range words {
			ch <- llm.StreamEvent{Token: w}
		}
		ch <- llm.StreamEvent{Done: true}
		close(ch)
		return ch
	}
}

// hang streams one token and then waits for the request to end.
func hang(tok string) step {
	return func(ctx context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent)
		go func() {
			defer close(ch)
			select {
			case ch <- llm.StreamEvent{Token: tok}:
			case <-ctx.Done():
				return
			}
			<-ctx.Done()
		}()
		return ch
	}
}

// script is the model: each request is answered by the next step.
type script struct {
	mu    sync.Mutex
	steps []step
	n     int
}

func (s *script) stream(ctx context.Context, _ []llm.Message, _ []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.n >= len(s.steps) {
		return answer("(the script ran out)")(ctx), nil
	}
	s.n++
	return s.steps[s.n-1](ctx), nil
}

// rig is the options a real hearthd would build, over a temp-dir store and a
// scripted model.
type rig struct {
	opts Options
	bus  *orchestrate.Bus
	root string // the Hearth folder a test names
	gw   *gateway
}

// gateway is every address an added service names, answering the models
// call with what a test sets, so no test leaves the process.
type gateway struct {
	mu     sync.Mutex
	status int
	body   string
}

func (g *gateway) set(status int, body string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.status, g.body = status, body
}

func (g *gateway) client() *http.Client {
	return &http.Client{Transport: gatewayFunc(func(*http.Request) (*http.Response, error) {
		g.mu.Lock()
		defer g.mu.Unlock()
		return &http.Response{StatusCode: g.status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(g.body))}, nil
	})}
}

type gatewayFunc func(*http.Request) (*http.Response, error)

func (f gatewayFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newRig(t *testing.T, steps ...step) *rig {
	t.Helper()
	provider.Init()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	gw := &gateway{status: http.StatusOK, body: `{"data":[{"id":"acme-large"}]}`}
	accounts, err := provider.Open(filepath.Join(dir, "data"), gw.client())
	if err != nil {
		t.Fatal(err)
	}
	bus := orchestrate.NewBus()
	sc := &script{steps: steps}
	svc, err := orchestrate.Open(t.Context(), orchestrate.Deps{
		Store:   db,
		Streams: func(org.Tier) (agent.Stream, error) { return sc.stream, nil },
		Bus:     bus,
	})
	if err != nil {
		t.Fatal(err)
	}
	return &rig{
		gw:   gw,
		bus:  bus,
		root: filepath.Join(dir, "Hearth"),
		opts: Options{
			Token:        testToken,
			Store:        db,
			Orchestrator: svc,
			Accounts:     accounts,
			Integrations: &integration.Integrations{Keys: accounts.Keys},
			Templates:    skill.Templates(),
			Streams:      func(org.Tier) (agent.Stream, error) { return sc.stream, nil },
			Bus:          bus,
		},
	}
}

// clients are the six services and the spike's, over one transport.
type clients struct {
	org   hearthv1connect.OrgServiceClient
	bot   hearthv1connect.BotServiceClient
	conv  hearthv1connect.ConversationServiceClient
	prov  hearthv1connect.ProviderServiceClient
	integ hearthv1connect.IntegrationServiceClient
	event hearthv1connect.EventServiceClient
}

func newClients(hc *http.Client, token string) clients {
	const base = "http://hearthd"
	opt := bearer(token)
	return clients{
		org:   hearthv1connect.NewOrgServiceClient(hc, base, opt),
		bot:   hearthv1connect.NewBotServiceClient(hc, base, opt),
		conv:  hearthv1connect.NewConversationServiceClient(hc, base, opt),
		prov:  hearthv1connect.NewProviderServiceClient(hc, base, opt),
		integ: hearthv1connect.NewIntegrationServiceClient(hc, base, opt),
		event: hearthv1connect.NewEventServiceClient(hc, base, opt),
	}
}

// follower reads the event stream a test subscribed to.
type follower struct {
	t  *testing.T
	ch chan *hearthv1.SubscribeResponse
}

// follow subscribes from sequence zero and returns once the core is
// sending: the stream's headers arrive with its first event, so it publishes
// heartbeats until one comes back, which proves the subscription is on the
// bus before the test acts. Heartbeats are skipped when reading.
func follow(t *testing.T, c clients, bus *orchestrate.Bus) *follower {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	f := &follower{t: t, ch: make(chan *hearthv1.SubscribeResponse, 256)}
	failed := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Subscribe returns when the headers do, which is with the first event.
		stream, err := c.event.Subscribe(ctx, connect.NewRequest(&hearthv1.SubscribeRequest{}))
		if err != nil {
			failed <- err
			return
		}
		defer stream.Close()
		for stream.Receive() {
			f.ch <- stream.Msg()
		}
	}()
	t.Cleanup(func() { cancel(); <-done })

	for {
		bus.Publish(orchestrate.Heartbeat{})
		select {
		case m := <-f.ch:
			if m.GetHeartbeat() == nil {
				t.Fatalf("first event is %v, want the heartbeat sent to prove the stream", m)
			}
			return f
		case err := <-failed:
			t.Fatalf("Subscribe: %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// until reads events, skipping heartbeats, until one satisfies stop, and
// returns them all.
func (f *follower) until(stop func(*hearthv1.SubscribeResponse) bool) []*hearthv1.SubscribeResponse {
	f.t.Helper()
	var got []*hearthv1.SubscribeResponse
	guard := time.NewTimer(10 * time.Second)
	defer guard.Stop()
	for {
		select {
		case m := <-f.ch:
			if m.GetHeartbeat() != nil {
				continue
			}
			got = append(got, m)
			if stop(m) {
				return got
			}
		case <-guard.C:
			f.t.Fatalf("the awaited event never came; saw %d", len(got))
		}
	}
}

// runEnded is true for a run_changed in a terminal state.
func runEnded(m *hearthv1.SubscribeResponse) bool {
	switch m.GetRunChanged().GetRun().GetState() {
	case hearthv1.RunState_RUN_STATE_DONE, hearthv1.RunState_RUN_STATE_FAILED, hearthv1.RunState_RUN_STATE_CANCELLED:
		return true
	}
	return false
}
