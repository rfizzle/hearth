package orchestrate

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// panicSearch is a search service that panics when asked.
type panicSearch struct{}

func (panicSearch) Search(context.Context, string, int) ([]web.Hit, error) { panic("boom") }

func TestRun_APanicEndsItFailedAndFreesTheConversation(t *testing.T) {
	// The first run may ask the model once more before it sees the stop, so
	// there are two replies for the second run to be sure of one.
	reply := events(llm.StreamEvent{Token: "Back again."}, llm.StreamEvent{Done: true})
	f := newFixture(t, events(toolCall("c1", "web.search", `{"query":"x"}`)), reply, reply)
	var logged bytes.Buffer
	f.svc.d.Log = log.New(&logged, "", 0)
	f.svc.d.Searcher = func() (web.Searcher, bool) { return panicSearch{}, true }

	r, _ := f.send(t, "search", ended)
	got := f.storedRun(t, r.ID)
	if got.State != org.RunFailed || got.Failure != "something went wrong inside hearth: boom" {
		t.Fatalf("run = %+v, want failed with the panic", got)
	}
	msgs := f.messages(t)
	if last := msgs[len(msgs)-1]; last.Kind != org.MessageSystem || last.Content != "Something went wrong inside hearth: boom." {
		t.Fatalf("last message = %+v, want the SYSTEM message", last)
	}
	if !strings.Contains(logged.String(), "boom") || !strings.Contains(logged.String(), "goroutine") {
		t.Errorf("log = %q, want the panic with its stack", logged.String())
	}
	r2, _ := f.send(t, "again", ended)
	if got := f.storedRun(t, r2.ID); got.State != org.RunDone {
		t.Fatalf("next run = %+v, want done", got)
	}
}

func TestOpen_ClosesStreamingMessagesAndDeletesEmptyOnes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	conv := f.bot.DMConversationID
	empty, err := f.db.AppendMessage(ctx, org.Message{ConversationID: conv, AuthorBotID: f.bot.ID, Kind: org.MessageText, Streaming: true})
	if err != nil {
		t.Fatal(err)
	}
	partial, err := f.db.AppendMessage(ctx, org.Message{ConversationID: conv, AuthorBotID: f.bot.ID, Kind: org.MessageText, Streaming: true, Content: "so far"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, Deps{Store: f.db, Streams: f.svc.d.Streams, Bus: f.bus}); err != nil {
		t.Fatal(err)
	}
	msgs := f.messages(t)
	if len(msgs) != 1 || msgs[0].ID != partial.ID || msgs[0].Streaming {
		t.Fatalf("messages = %+v, want only the partial, closed (empty %s deleted)", msgs, empty.ID)
	}
}

func TestRetry_AnswersTheLastOwnerMessageAgain(t *testing.T) {
	f := newFixture(t,
		events(llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}}),
		events(llm.StreamEvent{Token: "Here you go."}, llm.StreamEvent{Done: true}),
	)
	conv := f.bot.DMConversationID
	first, _ := f.send(t, "make me a brief", ended)
	if got := f.storedRun(t, first.ID); got.State != org.RunFailed {
		t.Fatalf("first run = %+v, want failed", got)
	}

	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	second, err := f.svc.Retry(context.Background(), conv)
	if err != nil {
		t.Fatal(err)
	}
	until(t, ch, ended)
	if second.ID == first.ID {
		t.Fatal("Retry reused the run")
	}
	if got := f.storedRun(t, second.ID); got.State != org.RunDone {
		t.Fatalf("second run = %+v, want done", got)
	}
	if got := f.storedRun(t, first.ID); got.State != org.RunFailed {
		t.Fatalf("first run = %+v, want untouched", got)
	}
	var owners, replies int
	for _, m := range f.messages(t) {
		switch {
		case m.AuthorBotID == "":
			owners++
		case m.RunID == second.ID && m.Content == "Here you go.":
			replies++
		}
	}
	if owners != 1 || replies != 1 {
		t.Fatalf("%d owner messages, %d replies on the new run, want 1 and 1: %+v", owners, replies, f.messages(t))
	}
	if latest, ok, err := f.db.LatestRun(context.Background(), conv); err != nil || !ok || latest.ID != second.ID {
		t.Fatalf("LatestRun = %+v, %v, %v, want the retry", latest, ok, err)
	}
}

func TestRetry_RefusesWhileARunIsWorking(t *testing.T) {
	f := newFixture(t, tokenThenHang("Thinking"))
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	r, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "go", nil)
	if err != nil {
		t.Fatal(err)
	}
	until(t, ch, isDelta)
	if _, err := f.svc.Retry(context.Background(), f.bot.DMConversationID); !errors.Is(err, ErrBusy) {
		t.Fatalf("Retry = %v, want ErrBusy", err)
	}
	f.svc.Cancel(r.ID)
	until(t, ch, ended)
}
