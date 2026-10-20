package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	hagent "github.com/rfizzle/hearth/internal/agent"
)

// script answers each request from the next step: events to stream, or a
// function for a stream that has to wait on its context.
type script struct {
	mu    sync.Mutex
	steps []func(ctx context.Context) <-chan llm.StreamEvent
	seen  [][]llm.Message
}

func (s *script) stream(ctx context.Context, msgs []llm.Message, _ []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = append(s.seen, append([]llm.Message(nil), msgs...))
	step := s.steps[len(s.seen)-1]
	return step(ctx), nil
}

func events(evs ...llm.StreamEvent) func(context.Context) <-chan llm.StreamEvent {
	return func(context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent, len(evs))
		for _, e := range evs {
			ch <- e
		}
		close(ch)
		return ch
	}
}

// tokenThenHang streams one token and then waits for the request to end.
func tokenThenHang(tok string) func(context.Context) <-chan llm.StreamEvent {
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

func toolRound(id, name, args string) func(context.Context) <-chan llm.StreamEvent {
	c := llm.ToolCall{ID: id, Name: name, Arguments: args}
	return events(llm.StreamEvent{Done: true, ToolCalls: []llm.ToolCall{c}})
}

// rec records what the sink was told, in order.
type rec struct {
	mu     sync.Mutex
	log    []string
	files  []string
	onText func(string)
}

func (r *rec) add(s string) {
	r.mu.Lock()
	r.log = append(r.log, s)
	r.mu.Unlock()
}

func (r *rec) Text(d string) {
	r.add("text:" + d)
	if r.onText != nil {
		r.onText(d)
	}
}
func (r *rec) ToolCall(id, name string, _ json.RawMessage) { r.add("call:" + id + ":" + name) }
func (r *rec) ToolResult(id string, res hagent.Result, err error, _ time.Duration) {
	s := "result:" + id + ":" + res.Text
	if err != nil {
		s += ":err=" + err.Error()
	}
	r.add(s)
	r.files = append(r.files, res.Files...)
}
func (r *rec) Round(n int)                        { r.add("round:" + string(rune('0'+n))) }
func (r *rec) Usage(string, string, hagent.Usage) {}
func (r *rec) Retry(a, m int, _ time.Duration) {
	r.add("retry:" + string(rune('0'+a)) + "/" + string(rune('0'+m)))
}

func echo(_ context.Context, name string, _ json.RawMessage) (hagent.Result, error) {
	return hagent.Result{Text: name + " done", Files: []string{"/f/" + name}}, nil
}

func TestRun(t *testing.T) {
	t.Run("two rounds record both results and the final text in order", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
			toolRound("c1", "one", `{}`),
			toolRound("c2", "two", `{}`),
			events(llm.StreamEvent{Token: "all"}, llm.StreamEvent{Token: " done"}, llm.StreamEvent{Done: true}),
		}}
		r := &rec{}
		out, err := hagent.Run(context.Background(), hagent.Input{
			System: "sys", Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 5, Sink: r,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		want := []string{
			"round:1", "call:c1:one", "result:c1:one done",
			"round:2", "call:c2:two", "result:c2:two done",
			"text:all", "text: done",
		}
		if strings.Join(r.log, "|") != strings.Join(want, "|") {
			t.Errorf("sink = %q, want %q", r.log, want)
		}
		if out.Text != "all done" || out.Rounds != 2 {
			t.Errorf("outcome = %+v", out)
		}
		if len(r.files) != 2 || r.files[0] != "/f/one" {
			t.Errorf("files = %q", r.files)
		}
	})

	t.Run("a round over the cap ends with the round cap error", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
			toolRound("c1", "one", `{}`),
			toolRound("c2", "two", `{}`),
		}}
		out, err := hagent.Run(context.Background(), hagent.Input{
			Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 1,
		})
		if !errors.Is(err, hagent.ErrRoundCap) {
			t.Fatalf("err = %v, want ErrRoundCap", err)
		}
		if errors.Is(err, hagent.ErrWallCap) || errors.Is(err, hagent.ErrCancelled) {
			t.Errorf("round cap is also another stop: %v", err)
		}
		if out.Rounds != 1 {
			t.Errorf("rounds = %d, want 1", out.Rounds)
		}
	})

	t.Run("cancelling mid-stream keeps the text and returns cancelled", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{tokenThenHang("half an ans")}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r := &rec{onText: func(string) { cancel() }}
		out, err := hagent.Run(ctx, hagent.Input{Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 5, Sink: r})
		if !errors.Is(err, hagent.ErrCancelled) {
			t.Fatalf("err = %v, want ErrCancelled", err)
		}
		if out.Text != "half an ans" {
			t.Errorf("kept text = %q", out.Text)
		}
	})

	t.Run("the wall cap stops a turn that outlasts it", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{tokenThenHang("slow")}}
		out, err := hagent.Run(context.Background(), hagent.Input{
			Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 5, MaxWall: 5 * time.Millisecond,
		})
		if !errors.Is(err, hagent.ErrWallCap) {
			t.Fatalf("err = %v, want ErrWallCap", err)
		}
		if out.Text != "slow" {
			t.Errorf("kept text = %q", out.Text)
		}
	})

	t.Run("an overloaded failure retries then succeeds", func(t *testing.T) {
		// The loop waits seconds between attempts; a synctest bubble runs
		// that wait on a fake clock.
		synctest.Test(t, func(t *testing.T) {
			overloaded := events(llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassOverloaded, Provider: "openai"}})
			s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
				overloaded,
				events(llm.StreamEvent{Token: "ok"}, llm.StreamEvent{Done: true}),
			}}
			r := &rec{}
			out, err := hagent.Run(context.Background(), hagent.Input{Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 5, Sink: r})
			if err != nil || out.Text != "ok" {
				t.Fatalf("out = %+v, err = %v", out, err)
			}
			if len(r.log) == 0 || r.log[0] != "retry:1/3" {
				t.Errorf("sink = %q, want a retry first", r.log)
			}
			if len(s.seen) != 2 {
				t.Errorf("requests = %d, want 2", len(s.seen))
			}
		})
	})

	t.Run("an auth failure ends at once", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
			events(llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}}),
		}}
		r := &rec{}
		_, err := hagent.Run(context.Background(), hagent.Input{Prompt: "go", Stream: s.stream, Execute: echo, MaxRounds: 5, Sink: r})
		if !errors.Is(err, failure.ErrAuth) {
			t.Fatalf("err = %v, want auth", err)
		}
		if len(s.seen) != 1 {
			t.Errorf("requests = %d, want 1 (no retry)", len(s.seen))
		}
		if got := hagent.Explain(err); !strings.Contains(got, "key was refused") {
			t.Errorf("Explain = %q", got)
		}
	})

	t.Run("a key in a tool result is scrubbed before the next request", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
			toolRound("c1", "peek", `{}`),
			events(llm.StreamEvent{Token: "ok"}, llm.StreamEvent{Done: true}),
		}}
		leak := func(context.Context, string, json.RawMessage) (hagent.Result, error) {
			return hagent.Result{Text: "token sk-SECRET here"}, nil
		}
		scrub := func(m hagent.Message) hagent.Message {
			m.Text = strings.ReplaceAll(m.Text, "sk-SECRET", "[redacted]")
			return m
		}
		r := &rec{}
		if _, err := hagent.Run(context.Background(), hagent.Input{
			Prompt: "go", Stream: s.stream, Execute: leak, MaxRounds: 5, Scrub: scrub, Sink: r,
		}); err != nil {
			t.Fatal(err)
		}
		for _, m := range s.seen[1] {
			if strings.Contains(m.Content, "sk-SECRET") {
				t.Errorf("request carried the key: %+v", m)
			}
		}
		if !strings.Contains(strings.Join(r.log, "|"), "[redacted]") || strings.Contains(strings.Join(r.log, "|"), "sk-SECRET") {
			t.Errorf("sink saw %q", r.log)
		}
	})

	t.Run("an executor error reaches the sink and the model reads it", func(t *testing.T) {
		s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
			toolRound("c1", "bad", `{}`),
			events(llm.StreamEvent{Token: "ok"}, llm.StreamEvent{Done: true}),
		}}
		bad := func(context.Context, string, json.RawMessage) (hagent.Result, error) {
			return hagent.Result{}, errors.New("no such folder")
		}
		r := &rec{}
		if _, err := hagent.Run(context.Background(), hagent.Input{Prompt: "go", Stream: s.stream, Execute: bad, MaxRounds: 5, Sink: r}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(r.log, "|"), "err=no such folder") {
			t.Errorf("sink = %q", r.log)
		}
		var read bool
		for _, m := range s.seen[1] {
			if m.Role == llm.RoleTool && strings.Contains(m.Content, "no such folder") {
				read = true
			}
		}
		if !read {
			t.Error("model did not read the error")
		}
	})
}

func TestRun_Park(t *testing.T) {
	gate := func(name string, _ json.RawMessage) bool { return strings.HasPrefix(name, "ask.") }
	cases := []struct {
		name  string
		calls []llm.ToolCall
		want  []string // what the sink is told
	}{
		{
			"one auto call and one gated call",
			[]llm.ToolCall{{ID: "c1", Name: "one", Arguments: `{}`}, {ID: "c2", Name: "ask.delete", Arguments: `{"path":"a.md"}`}},
			[]string{"round:1", "call:c1:one", "result:c1:one done", "call:c2:ask.delete", "result:c2::err=" + hagent.ErrParked.Error()},
		},
		{
			"two gated calls park on the first and owe the second nothing",
			[]llm.ToolCall{{ID: "c1", Name: "ask.delete", Arguments: `{"path":"a.md"}`}, {ID: "c2", Name: "ask.send", Arguments: `{}`}},
			[]string{"round:1", "call:c1:ask.delete", "result:c1::err=" + hagent.ErrParked.Error(), "call:c2:ask.send", "result:c2::err=" + hagent.ErrParked.Error()},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
				events(llm.StreamEvent{Token: "Let me tidy up."}, llm.StreamEvent{Done: true, ToolCalls: tc.calls}),
				events(llm.StreamEvent{Token: "never asked"}, llm.StreamEvent{Done: true}),
			}}
			r := &rec{}
			out, err := hagent.Run(context.Background(), hagent.Input{
				Prompt: "tidy", Stream: s.stream, Execute: echo, Gate: gate, MaxRounds: 5, Sink: r,
			})
			if !errors.Is(err, hagent.ErrParked) {
				t.Fatalf("err = %v, want ErrParked", err)
			}
			// The first gated call is the one the turn waits on.
			first := tc.calls[0]
			if !gate(first.Name, nil) {
				first = tc.calls[1]
			}
			if out.Parked == nil || out.Parked.ID != first.ID || out.Parked.Name != first.Name || string(out.Parked.Args) != first.Arguments {
				t.Fatalf("parked = %+v, want %+v", out.Parked, first)
			}
			log := strings.Join(r.log, "|")
			if want := strings.Join(tc.want, "|"); !strings.HasSuffix(log, want) {
				t.Errorf("sink = %q, want it to end %q", r.log, tc.want)
			}
			if strings.Contains(log, "result:c1:waiting") || strings.Contains(log, "result:c2:waiting") {
				t.Errorf("the sink was handed the placeholder: %q", r.log)
			}
			if len(s.seen) != 1 {
				t.Errorf("requests = %d, want 1: a parked turn asks the model nothing more", len(s.seen))
			}
		})
	}
}

func TestRun_HistoryCarriesCallsAndResults(t *testing.T) {
	s := &script{steps: []func(context.Context) <-chan llm.StreamEvent{
		events(llm.StreamEvent{Token: "ok"}, llm.StreamEvent{Done: true}),
	}}
	history := []hagent.Message{
		{Role: hagent.RoleOwner, Text: "tidy up"},
		{Role: hagent.RoleBot, Text: "On it.", Calls: []hagent.ToolCall{{ID: "c1", Name: "files.delete", Args: json.RawMessage(`{"path":"a.md"}`)}}},
		{Role: hagent.RoleTool, CallID: "c1", Text: "Deleted a.md."},
	}
	if _, err := hagent.Run(context.Background(), hagent.Input{History: history, Prompt: "(hearth) carry on", Stream: s.stream, MaxRounds: 5}); err != nil {
		t.Fatal(err)
	}
	sent := s.seen[0]
	if len(sent) != 4 {
		t.Fatalf("sent %d messages, want history and prompt: %+v", len(sent), sent)
	}
	bot, tool := sent[1], sent[2]
	if bot.Role != llm.RoleAssistant || bot.Content != "On it." || len(bot.ToolCalls) != 1 ||
		bot.ToolCalls[0] != (llm.ToolCall{ID: "c1", Name: "files.delete", Arguments: `{"path":"a.md"}`}) {
		t.Errorf("bot message = %+v", bot)
	}
	if tool.Role != llm.RoleTool || tool.ToolCallID != "c1" || tool.Content != "Deleted a.md." {
		t.Errorf("tool message = %+v", tool)
	}
}

func TestExplain(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&failure.Failure{Class: failure.ClassNetwork, Provider: "openai"}, "could not reach OpenAI"},
		{&failure.Failure{Class: failure.ClassAuth, Provider: "openai"}, "the key was refused"},
		{&failure.Failure{Class: failure.ClassContextLength}, "the model's limit was hit"},
		{&failure.Failure{Class: failure.ClassQuota, Provider: "openai"}, "the model's limit was hit"},
		{hagent.ErrRoundCap, "steps"},
		{hagent.ErrWallCap, "time"},
		{errors.New("odd"), "odd"},
	} {
		if got := hagent.Explain(tc.err); !strings.Contains(got, tc.want) {
			t.Errorf("Explain(%v) = %q, want it to contain %q", tc.err, got, tc.want)
		}
	}
}
