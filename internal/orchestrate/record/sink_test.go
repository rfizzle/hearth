package record

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/orchestrate/toolset"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/files"
)

// fixture is a host over a temp-dir store with one hired bot, and a record
// of the conversations the host was asked to release.
type fixture struct {
	db   *store.DB
	bus  *bus.Bus
	bot  org.Bot
	host Host

	mu       sync.Mutex
	released []string // "conversation/run", in order
	states   []org.RunState
}

func newFixture(t *testing.T) *fixture {
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
	f := &fixture{db: db, bus: bus.New(), bot: bot}
	f.host = Host{Store: db, Bus: f.bus, Log: log.New(os.Stderr, "", 0), Now: time.Now,
		Release: func(conv, run string) {
			// What the store holds when the conversation is freed: the
			// run's final state is already there.
			r, err := db.Run(ctx, run)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.released = append(f.released, conv+"/"+run)
			if err == nil {
				f.states = append(f.states, r.State)
			}
		}}
	return f
}

// newSink is a sink on a working run of the fixture's bot.
func newSink(t *testing.T, f *fixture) *Sink {
	t.Helper()
	ctx := context.Background()
	r, err := f.db.CreateRun(ctx, org.Run{ConversationID: f.bot.DMConversationID, BotID: f.bot.ID, State: org.RunWorking})
	if err != nil {
		t.Fatal(err)
	}
	return New(f.host, ctx, r, f.bot.ID, f.bot.Name)
}

func (f *fixture) messages(t *testing.T) []org.Message {
	t.Helper()
	msgs, _, err := f.db.Messages(context.Background(), f.bot.DMConversationID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

// until reads events until one satisfies stop, and returns them all.
func until(t *testing.T, ch <-chan bus.Event, stop func(bus.Event) bool) []bus.Event {
	t.Helper()
	var got []bus.Event
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
			t.Fatalf("no closing event; saw %d", len(got))
		}
	}
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
	k.Finish(agent.Outcome{Text: "Fresh"}, nil)
	msgs := f.messages(t)
	if len(msgs) != 1 || msgs[0].Content != "Fresh" || msgs[0].Streaming {
		t.Fatalf("messages = %+v, want only the retried reply", msgs)
	}
	// The conversation is freed after the final state is stored, and once.
	if len(f.released) != 1 || f.released[0] != r.ConversationID+"/"+r.ID || f.states[0] != org.RunDone {
		t.Fatalf("released %v at states %v, want the run's conversation once, with the run stored done", f.released, f.states)
	}
}

func TestSink_AFailedRetryLeavesNoEmptyBubble(t *testing.T) {
	f := newFixture(t)
	k := newSink(t, f)
	k.Text("from the failed attempt")
	k.Retry(1, 3, 0)
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	k.Finish(agent.Outcome{}, errors.New("it broke"))
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageText {
			t.Fatalf("messages = %+v, want no empty bot text", f.messages(t))
		}
	}
	evs := until(t, ch, func(e bus.Event) bool { _, ok := e.Body.(bus.MessageDeleted); return ok })
	if len(evs) == 0 {
		t.Fatal("no MessageDeleted published")
	}
}

func TestSink_ACrashIsRecordedOnceAndStopsTheTurn(t *testing.T) {
	f := newFixture(t)
	k := newSink(t, f)
	stopped := 0
	k.SetStop(func() { stopped++ })
	if k.Crashed() != nil {
		t.Fatal("a sink that has not crashed reports a panic")
	}
	k.Crash("first")
	k.Crash("second")
	if k.Crashed() != "first" || stopped != 2 {
		t.Fatalf("Crashed = %v after %d stops, want the first panic and a stop each time", k.Crashed(), stopped)
	}
}

func TestSink_EquipSummarisesTheCallsTheDispatcherPlaces(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.bot.FolderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	reg, err := toolset.Registry(files.Options{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	grants, err := f.db.GrantsFor(context.Background(), f.bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	k := newSink(t, f)
	k.Equip(tools.Dispatcher{Defs: reg, Grants: grants, Root: f.bot.FolderPath})
	if len(k.describe) == 0 {
		t.Fatal("Equip kept no describer of the tools offered")
	}
	k.ToolCall("c1", "files.write", json.RawMessage(`{"path":"a.md","content":"x"}`))
	if got := f.messages(t)[0].ToolCalls[0].Summary; got == "" {
		t.Fatal("the call has no plain-words summary")
	}
}

func TestSentence_CapitalisesAndEndsWithAFullStop(t *testing.T) {
	for in, want := range map[string]string{
		"the key was refused": "The key was refused.",
		"Already done.":       "Already done.",
		"émile left":          "Émile left.",
	} {
		if got := Sentence(in); got != want {
			t.Errorf("Sentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSink_StartMarksTheRunWorkingAndPublishesIt(t *testing.T) {
	f := newFixture(t)
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	f.host.Now = func() time.Time { return at }
	r, err := f.db.CreateRun(context.Background(), org.Run{ConversationID: f.bot.DMConversationID, BotID: f.bot.ID, State: org.RunQueued})
	if err != nil {
		t.Fatal(err)
	}
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	New(f.host, context.Background(), r, f.bot.ID, f.bot.Name).Start()
	evs := until(t, ch, func(e bus.Event) bool { _, ok := e.Body.(bus.RunChanged); return ok })
	if got := evs[len(evs)-1].Body.(bus.RunChanged).Run; got.State != org.RunWorking || !got.StartedAt.Equal(at) {
		t.Fatalf("published run = %+v, want working from %v", got, at)
	}
	if stored, err := f.db.Run(context.Background(), r.ID); err != nil || stored.State != org.RunWorking {
		t.Fatalf("stored run = %+v, %v, want working", stored, err)
	}
}
