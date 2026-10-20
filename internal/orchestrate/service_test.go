package orchestrate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
)

// step answers one request of a scripted stream.
type step func(ctx context.Context) <-chan llm.StreamEvent

// script answers each request from the next step and records what the
// model was sent.
type script struct {
	mu    sync.Mutex
	steps []step
	seen  [][]llm.Message
}

func (s *script) stream(ctx context.Context, msgs []llm.Message, _ []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = append(s.seen, append([]llm.Message(nil), msgs...))
	if len(s.seen) > len(s.steps) {
		return nil, errors.New("the script has no more steps")
	}
	return s.steps[len(s.seen)-1](ctx), nil
}

func events(evs ...llm.StreamEvent) step {
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
func tokenThenHang(tok string) step {
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

func toolCall(id, name, args string) llm.StreamEvent {
	return llm.StreamEvent{Done: true, ToolCalls: []llm.ToolCall{{ID: id, Name: name, Arguments: args}}}
}

// fixture is a service over a temp-dir store with one hired bot whose folder
// does not exist yet.
type fixture struct {
	db     *store.DB
	bus    *Bus
	svc    *Service
	bot    org.Bot
	script *script
}

func newFixture(t *testing.T, steps ...step) *fixture {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	root := filepath.Join(dir, "Hearth")
	if err := db.PutOrg(ctx, org.Org{Name: "Acme", WorkspacePath: root}); err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Finds things out.", org.TierBalanced, root)
	if bot, err = db.CreateBot(ctx, bot, grants, dm); err != nil {
		t.Fatal(err)
	}
	f := &fixture{db: db, bus: NewBus(), bot: bot, script: &script{steps: steps}}
	streams := func(org.Tier) (agent.Stream, error) { return f.script.stream, nil }
	if f.svc, err = Open(ctx, Deps{Store: db, Streams: streams, Bus: f.bus}); err != nil {
		t.Fatal(err)
	}
	return f
}

// until reads events until one satisfies stop, and returns them all.
func until(t *testing.T, ch <-chan Event, stop func(Event) bool) []Event {
	t.Helper()
	var got []Event
	guard := time.NewTimer(10 * time.Second)
	defer guard.Stop()
	for {
		select {
		case e := <-ch:
			got = append(got, e)
			if stop(e) {
				return got
			}
		case <-guard.C:
			t.Fatalf("no closing event; saw %s", labels(got))
		}
	}
}

func ended(e Event) bool {
	rc, ok := e.Body.(RunChanged)
	return ok && rc.Run.State != org.RunQueued && rc.Run.State != org.RunWorking
}

func isDelta(e Event) bool {
	_, ok := e.Body.(MessageDelta)
	return ok
}

// send sends text to the bot's DM and returns the run and the events from
// that point until stop.
func (f *fixture) send(t *testing.T, text string, stop func(Event) bool) (org.Run, []Event) {
	t.Helper()
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	r, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, text)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	return r, until(t, ch, stop)
}

func (f *fixture) messages(t *testing.T) []org.Message {
	t.Helper()
	msgs, _, err := f.db.Messages(context.Background(), f.bot.DMConversationID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func (f *fixture) storedRun(t *testing.T, id string) org.Run {
	t.Helper()
	r, err := f.db.Run(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// labels names each event by what the app does with it.
func labels(evs []Event) string {
	var out []string
	for _, e := range evs {
		switch b := e.Body.(type) {
		case MessageDelta:
			out = append(out, "delta "+b.Text)
		case MessageUpserted:
			m := b.Message
			l := map[org.MessageKind]string{org.MessageText: "text", org.MessageToolCall: "tool_call",
				org.MessageToolResult: "tool_result", org.MessageSystem: "system"}[m.Kind]
			switch {
			case m.AuthorBotID == "":
				l = "owner " + l
			case m.Streaming:
				l += " streaming " + fmt.Sprintf("%q", m.Content)
			case m.Kind == org.MessageText:
				l += " " + fmt.Sprintf("%q", m.Content)
			}
			out = append(out, l)
		case RunChanged:
			out = append(out, fmt.Sprintf("run %d steps %d", b.Run.State, b.Run.Steps))
		default:
			out = append(out, fmt.Sprintf("%T", b))
		}
	}
	return strings.Join(out, " | ")
}

func TestService(t *testing.T) {
	t.Run("a turn that writes a file leaves it on disk and the run done", func(t *testing.T) {
		f := newFixture(t,
			events(toolCall("c1", "files.write", `{"path":"brief.md","content":"# Brief\n"}`)),
			events(llm.StreamEvent{Token: "Saved brief.md."}, llm.StreamEvent{Done: true}),
		)
		r, _ := f.send(t, "Write me a brief.", ended)

		path := filepath.Join(f.bot.FolderPath, "brief.md")
		if b, err := os.ReadFile(path); err != nil || string(b) != "# Brief\n" {
			t.Fatalf("brief.md = %q, %v", b, err)
		}
		msgs := f.messages(t)
		if len(msgs) != 4 {
			t.Fatalf("%d messages, want owner, call, result, reply: %+v", len(msgs), msgs)
		}
		call, result, reply := msgs[1], msgs[2], msgs[3]
		if call.Kind != org.MessageToolCall || call.ToolCalls[0].Summary == "" || call.AuthorBotID != f.bot.ID {
			t.Errorf("call = %+v, want a tool call with a summary", call)
		}
		want := org.FileRef{Path: path, Name: "brief.md"}
		if result.Kind != org.MessageToolResult || result.ToolCallID != "c1" || len(result.Files) != 1 || result.Files[0] != want {
			t.Errorf("result = %+v, want the call's result with %+v", result, want)
		}
		if reply.Kind != org.MessageText || reply.Content != "Saved brief.md." || reply.Streaming {
			t.Errorf("reply = %+v", reply)
		}
		got := f.storedRun(t, r.ID)
		if got.State != org.RunDone || got.Steps != 1 || got.StartedAt.IsZero() || got.EndedAt.IsZero() {
			t.Errorf("run = %+v, want done after 1 step", got)
		}
	})

	t.Run("a second Send during the run returns ErrBusy", func(t *testing.T) {
		f := newFixture(t, tokenThenHang("Thinking"))
		ch, cancel := f.bus.Subscribe(0)
		defer cancel()
		r, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "first")
		if err != nil {
			t.Fatal(err)
		}
		until(t, ch, isDelta)
		if _, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "second"); !errors.Is(err, ErrBusy) {
			t.Fatalf("second Send = %v, want ErrBusy", err)
		}
		f.svc.Cancel(r.ID)
		until(t, ch, ended)
		if n := len(f.messages(t)); n != 2 {
			t.Fatalf("%d messages, want the first message and the partial reply only", n)
		}
	})

	t.Run("Cancel mid-stream keeps the partial text and ends the run cancelled", func(t *testing.T) {
		f := newFixture(t, tokenThenHang("Half a"))
		ch, cancel := f.bus.Subscribe(0)
		defer cancel()
		r, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "go")
		if err != nil {
			t.Fatal(err)
		}
		until(t, ch, isDelta)
		if !f.svc.Cancel(r.ID) {
			t.Fatal("Cancel found no working run")
		}
		until(t, ch, ended)
		msgs := f.messages(t)
		if last := msgs[len(msgs)-1]; last.Kind != org.MessageText || last.Content != "Half a" || last.Streaming {
			t.Errorf("last message = %+v, want the partial reply, no longer streaming", last)
		}
		if got := f.storedRun(t, r.ID); got.State != org.RunCancelled || got.Failure != "" {
			t.Errorf("run = %+v, want cancelled", got)
		}
		if f.svc.Cancel(r.ID) {
			t.Error("Cancel stopped a run that had already ended")
		}
		if _, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "again"); err != nil {
			t.Errorf("Send after the run ended = %v", err)
		}
	})

	t.Run("a provider auth failure leaves a SYSTEM message and the run failed", func(t *testing.T) {
		f := newFixture(t, events(llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}}))
		r, evs := f.send(t, "go", ended)
		msgs := f.messages(t)
		sys := msgs[len(msgs)-1]
		if sys.Kind != org.MessageSystem || !strings.HasPrefix(sys.Content, "The key was refused by OpenAI") {
			t.Fatalf("last message = %+v, want the reason as a system message", sys)
		}
		if sys.AuthorBotID != f.bot.ID || sys.RunID != r.ID {
			t.Errorf("system message = %+v, want it on the bot's run", sys)
		}
		got := f.storedRun(t, r.ID)
		if got.State != org.RunFailed || !strings.Contains(got.Failure, "the key was refused") {
			t.Errorf("run = %+v, want failed with the reason", got)
		}
		if l := labels(evs); !strings.HasSuffix(l, "system | run 5 steps 0") {
			t.Errorf("events = %s, want the system message before the failed run", l)
		}
	})

	t.Run("a subscriber from sequence 0 sees the reply's events in order", func(t *testing.T) {
		f := newFixture(t,
			events(llm.StreamEvent{Token: "On it."}, toolCall("c1", "files.write", `{"path":"a.md","content":"a"}`)),
			events(llm.StreamEvent{Token: "Done"}, llm.StreamEvent{Token: "."}, llm.StreamEvent{Done: true}),
		)
		_, evs := f.send(t, "go", ended)
		want := strings.Join([]string{
			"owner text",
			"run 2 steps 0",
			`text streaming ""`, "delta On it.", `text "On it."`,
			"run 2 steps 1",
			"tool_call", "tool_result",
			`text streaming ""`, "delta Done", "delta .", `text "Done."`,
			"run 4 steps 1",
		}, " | ")
		if got := labels(evs); got != want {
			t.Fatalf("events:\n got %s\nwant %s", got, want)
		}
		for i := 1; i < len(evs); i++ {
			if evs[i].Seq != evs[i-1].Seq+1 {
				t.Fatalf("sequence jumps from %d to %d", evs[i-1].Seq, evs[i].Seq)
			}
		}
	})

	t.Run("the next message is sent the earlier exchange without its tool rows", func(t *testing.T) {
		f := newFixture(t,
			events(toolCall("c1", "files.write", `{"path":"a.md","content":"a"}`)),
			events(llm.StreamEvent{Token: "Wrote it."}, llm.StreamEvent{Done: true}),
			events(llm.StreamEvent{Token: "Sure."}, llm.StreamEvent{Done: true}),
		)
		f.send(t, "write a", ended)
		f.send(t, "thanks", ended)
		sent := f.script.seen[2]
		var got []string
		for _, m := range sent[1:] {
			got = append(got, string(m.Role)+":"+m.Content)
		}
		want := "user:write a | assistant:Wrote it. | user:thanks"
		if strings.Join(got, " | ") != want || sent[0].Role != llm.RoleSystem {
			t.Fatalf("sent %q, want the framing then %s", got, want)
		}
	})
}

func TestOpen_FailsRunsAnEarlierLaunchLeftWorking(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	left, err := f.db.CreateRun(ctx, org.Run{ConversationID: f.bot.DMConversationID, BotID: f.bot.ID, State: org.RunWorking})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, Deps{Store: f.db, Streams: f.svc.d.Streams, Bus: f.bus}); err != nil {
		t.Fatal(err)
	}
	got := f.storedRun(t, left.ID)
	if got.State != org.RunFailed || got.Failure != "hearth was quit while this was running" {
		t.Fatalf("run = %+v, want failed by the sweep", got)
	}
}

// newSink is a sink on a working run of the fixture's bot.
func newSink(t *testing.T, f *fixture) *sink {
	t.Helper()
	ctx := context.Background()
	r, err := f.db.CreateRun(ctx, org.Run{ConversationID: f.bot.DMConversationID, BotID: f.bot.ID, State: org.RunWorking})
	if err != nil {
		t.Fatal(err)
	}
	f.svc.working[r.ConversationID] = r.ID
	return &sink{s: f.svc, ctx: ctx, run: r, botID: f.bot.ID, calls: map[string]org.ToolCall{},
		describe: map[string]func(json.RawMessage) string{}}
}

func TestSink_AToolRoundClosesTheTextBeforeIt(t *testing.T) {
	f := newFixture(t)
	k := newSink(t, f)
	k.describe["files.write"] = func(json.RawMessage) string { return "Write a.md" }
	k.Text("Let me write it.")
	k.Usage("chatgpt", "gpt", agent.Usage{Prompt: 10}) // not recorded yet: no message
	k.Round(1)
	k.ToolCall("c1", "files.write", json.RawMessage(`{}`))
	k.ToolResult("c1", agent.Result{Text: "Wrote a.md.", Files: []string{"/w/a.md"}}, nil, time.Second)
	k.ToolCall("c2", "files.write", json.RawMessage(`{}`))
	k.ToolResult("c2", agent.Result{}, errors.New("files.write refused: outside"), time.Second)
	msgs := f.messages(t)
	if len(msgs) != 5 || msgs[0].Content != "Let me write it." || msgs[0].Streaming {
		t.Fatalf("messages = %+v, want the closed text first", msgs)
	}
	if msgs[1].ToolCalls[0].Summary != "Write a.md" || msgs[2].ToolCalls[0].Summary != "Write a.md" {
		t.Errorf("call and result = %+v, %+v, want the summary on both", msgs[1], msgs[2])
	}
	if msgs[2].Files[0] != (org.FileRef{Path: "/w/a.md", Name: "a.md"}) {
		t.Errorf("result files = %+v", msgs[2].Files)
	}
	if msgs[4].Content != "files.write refused: outside" {
		t.Errorf("failed result = %q, want the error", msgs[4].Content)
	}
	if k.run.Steps != 1 {
		t.Errorf("steps = %d, want 1", k.run.Steps)
	}
}

func TestSink_RetryEmptiesTheStreamingMessage(t *testing.T) {
	f := newFixture(t)
	k := newSink(t, f)
	r := k.run
	k.Text("from the failed attempt")
	k.Retry(1, 3, time.Second)
	k.Text("Fresh")
	k.finish(agent.Outcome{Text: "Fresh"}, nil)
	msgs := f.messages(t)
	if len(msgs) != 1 || msgs[0].Content != "Fresh" || msgs[0].Streaming {
		t.Fatalf("messages = %+v, want only the retried reply", msgs)
	}
	if _, busy := f.svc.working[r.ConversationID]; busy {
		t.Fatal("the conversation is still busy after the run ended")
	}
}
