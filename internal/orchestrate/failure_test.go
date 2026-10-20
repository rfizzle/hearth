package orchestrate

import (
	"context"
	"errors"
	"testing"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

func refusedKey() step {
	return events(llm.StreamEvent{Err: &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}})
}

// failed sends text to a bot whose provider refuses it, and returns the run
// and the item the failure opened.
func (f *fixture) failed(t *testing.T, text string) (org.Run, inbox.Item) {
	t.Helper()
	r, evs := f.send(t, text, func(e Event) bool { _, ok := e.Body.(InboxChanged); return ok })
	it := evs[len(evs)-1].Body.(InboxChanged).Item
	if got := f.storedRun(t, r.ID); got.State != org.RunFailed {
		t.Fatalf("run = %+v, want failed", got)
	}
	return r, it
}

func openFailures(t *testing.T, f *fixture) []inbox.Item {
	t.Helper()
	items, err := f.db.InboxItems(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestFailure_AProviderFailureIsAnInboxItem(t *testing.T) {
	f := newFixture(t, refusedKey(), events(llm.StreamEvent{Token: "Hello."}, llm.StreamEvent{Done: true}))
	r, it := f.failed(t, "make me a brief")
	if it.Kind != inbox.Failure || it.State != inbox.Open || it.RunID != r.ID || it.BotID != f.bot.ID ||
		it.Title != "Dana: the key was refused by OpenAI; sign in again or replace it" {
		t.Fatalf("item = %+v", it)
	}
	if items := openFailures(t, f); len(items) != 1 || items[0].ID != it.ID {
		t.Fatalf("open items = %+v, want the failure", items)
	}

	ctx := context.Background()
	closed, err := f.svc.Acknowledge(ctx, it.ID)
	if err != nil || closed.State != inbox.Closed || closed.Outcome != "Acknowledged" {
		t.Fatalf("Acknowledge = %+v, %v", closed, err)
	}
	if _, err := f.svc.Acknowledge(ctx, it.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("a second Acknowledge = %v, want ErrClosed", err)
	}
	if items := openFailures(t, f); len(items) != 0 {
		t.Fatalf("open items = %+v, want none", items)
	}
}

func TestFailure_ANewMessageIsSentAndClosesTheItem(t *testing.T) {
	f := newFixture(t, refusedKey(), events(llm.StreamEvent{Token: "Hello."}, llm.StreamEvent{Done: true}))
	_, it := f.failed(t, "first")
	// The item holds no conversation: the owner can write to the bot, and
	// the run that answers them supersedes the failure, so no Retry on it
	// can send their newest message again.
	if next, _ := f.send(t, "second", ended); f.storedRun(t, next.ID).State != org.RunDone {
		t.Fatal("the failure item blocked the next message")
	}
	got, err := f.db.InboxItem(context.Background(), it.ID)
	if err != nil || got.State != inbox.Closed || got.Outcome != "Closed: a new message was sent" {
		t.Fatalf("item = %+v, %v, want closed by the new message", got, err)
	}
	if items := openFailures(t, f); len(items) != 0 {
		t.Fatalf("open items = %+v, want none", items)
	}
}

func TestFailure_RetryClosesTheItem(t *testing.T) {
	f := newFixture(t, refusedKey(), events(llm.StreamEvent{Token: "Here you go."}, llm.StreamEvent{Done: true}))
	_, it := f.failed(t, "make me a brief")
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	retry, err := f.svc.Retry(context.Background(), f.bot.DMConversationID)
	if err != nil {
		t.Fatal(err)
	}
	until(t, ch, runEnded(retry.ID))
	if got := f.storedRun(t, retry.ID); got.State != org.RunDone {
		t.Fatalf("retry = %+v, want done", got)
	}
	got, err := f.db.InboxItem(context.Background(), it.ID)
	if err != nil || got.State != inbox.Closed || got.Outcome != "Retried" {
		t.Fatalf("item = %+v, %v, want closed by the retry", got, err)
	}
	sent := f.script.seen[1]
	if last := sent[len(sent)-1]; last.Role != llm.RoleUser || last.Content != "make me a brief" {
		t.Fatalf("the retry ended its request with %+v, want the owner's last message again", last)
	}
}

func TestFailure_OnlyAProviderFailureLeavesAnItem(t *testing.T) {
	// The script has no step to answer with: the run fails on hearth's side.
	f := newFixture(t)
	r, _ := f.send(t, "hello", ended)
	if got := f.storedRun(t, r.ID); got.State != org.RunFailed {
		t.Fatalf("run = %+v, want failed", got)
	}
	if items := openFailures(t, f); len(items) != 0 {
		t.Fatalf("open items = %+v, want none", items)
	}
}
