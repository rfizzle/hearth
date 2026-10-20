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

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
)

// ledger is a meter that keeps what it is told and pauses nothing.
type ledger struct {
	mu   sync.Mutex
	recs []usage.Record
}

func (l *ledger) Record(_ context.Context, r usage.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recs = append(l.recs, r)
	return nil
}

func (l *ledger) Paused(context.Context, string) (bool, error) { return false, nil }

// runs is how many requests the ledger holds for each run.
func (l *ledger) runs() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := map[string]int{}
	for _, r := range l.recs {
		out[r.RunID]++
	}
	return out
}

// withLedger is f's service again, metered by a ledger.
func withLedger(t *testing.T, f *fixture) *ledger {
	t.Helper()
	l := &ledger{}
	d := f.svc.d
	d.Meter = l
	svc, err := Open(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return l
}

// reply streams text after reporting a request's usage.
func reply(text string) step {
	return events(llm.StreamEvent{Usage: &llm.Usage{PromptTokens: 10}}, llm.StreamEvent{Token: text}, llm.StreamEvent{Done: true})
}

func TestSend_CopiesADroppedFileIntoTheInbox(t *testing.T) {
	f := newFixture(t, reply("Got it."))
	ctx := context.Background()
	desktop := filepath.Join(t.TempDir(), "Desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	dropped := filepath.Join(desktop, "a.txt")
	if err := os.WriteFile(dropped, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{"a folder": desktop, "a missing file": filepath.Join(desktop, "nope.txt")} {
		if _, _, err := f.svc.Send(ctx, f.bot.DMConversationID, "look", []string{p}); !errors.Is(err, ErrAttachment) {
			t.Fatalf("Send with %s = %v, want ErrAttachment", name, err)
		}
	}
	if msgs := f.messages(t); len(msgs) != 0 {
		t.Fatalf("a refused message was stored: %+v", msgs)
	}

	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	_, m, err := f.svc.Send(ctx, f.bot.DMConversationID, "look", []string{dropped})
	if err != nil {
		t.Fatalf("Send after a refusal: %v", err)
	}
	until(t, ch, ended)
	o, _ := f.db.Org(ctx)
	copied := filepath.Join(o.WorkspacePath, "Inbox", "a.txt")
	if len(m.Attachments) != 1 || m.Attachments[0] != (org.FileRef{Path: copied, Name: "a.txt"}) {
		t.Fatalf("attachments = %+v, want the copy in the Inbox", m.Attachments)
	}
	if stored := f.messages(t)[0]; len(stored.Attachments) != 1 || stored.Attachments[0].Path != copied {
		t.Errorf("stored owner message = %+v, want it to carry the ref", stored)
	}
	if b, err := os.ReadFile(copied); err != nil || string(b) != "hello" {
		t.Errorf("copy = %q, %v", b, err)
	}
	sent := f.script.seen[0]
	if got, want := sent[len(sent)-1].Content, "look\n\nFile attached: a.txt (5 bytes) at "+copied; got != want {
		t.Errorf("prompt = %q, want %q", got, want)
	}
}

func TestDispatcher_TheInboxIsReadOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	o, _ := f.db.Org(ctx)
	box := filepath.Join(o.WorkspacePath, "Inbox")
	if err := os.MkdirAll(box, 0o755); err != nil {
		t.Fatal(err)
	}
	disp, err := f.svc.dispatcher(ctx, f.bot, f.bot.DMConversationID, "")
	if err != nil {
		t.Fatal(err)
	}
	args := json.RawMessage(fmt.Sprintf(`{"path":%q,"content":"x"}`, filepath.Join(box, "a.txt")))
	if v, err := disp.Decide("files.read", args); err != nil || v != 1 {
		t.Errorf("files.read in the Inbox = %v, %v; want it run without asking", v, err)
	}
	for _, name := range []string{"files.write", "files.delete"} {
		if _, err := disp.Check(name, args); err == nil {
			t.Errorf("%s in the Inbox was let through", name)
		}
	}
	if grants, _ := f.db.GrantsFor(ctx, f.bot.ID); len(grants) != len(disp.Grants)-1 {
		t.Errorf("the read on the Inbox was stored: %d grants, dispatcher has %d", len(grants), len(disp.Grants))
	}
}

func TestAfter_ALongConversationIsSummarisedAndMetered(t *testing.T) {
	f := newFixture(t, reply("ok"), reply("The person and Dana talked."))
	l := withLedger(t, f)
	ctx := context.Background()
	for i := range 60 { // 60 KiB against Balanced's 48
		m := org.Message{ConversationID: f.bot.DMConversationID, Kind: org.MessageText, Content: fmt.Sprintf("<m%02d>", i) + strings.Repeat("x", 1<<10)}
		if i%2 == 1 {
			m.AuthorBotID = f.bot.ID
		}
		if _, err := f.db.AppendMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	r, _ := f.send(t, "and?", func(e Event) bool { _, ok := e.Body.(ConversationChanged); return ok })
	c, err := f.db.Conversation(ctx, f.bot.DMConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Summary != "The person and Dana talked." || c.SummaryThroughSeq < 1 || c.SummaryThroughSeq >= 60 {
		t.Errorf("summary = %q through %d", c.Summary, c.SummaryThroughSeq)
	}
	if !strings.Contains(f.script.seen[1][1].Content, "<m00>") {
		t.Errorf("the summary request did not hold the oldest message: %.200q", f.script.seen[1][1].Content)
	}
	if got := l.runs()[r.ID]; got != 2 {
		t.Errorf("the ledger holds %d requests for the run, want the run's and the summary's", got)
	}
}

func TestAfter_FullNotesAreTidied(t *testing.T) {
	f := newFixture(t, reply("ok"), reply(`["likes tea", "works Tuesdays"]`))
	l := withLedger(t, f)
	ctx := context.Background()
	for i := range memory.Cap {
		if _, err := f.db.AddNote(ctx, memory.Note{BotID: f.bot.ID, Text: fmt.Sprintf("note %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	tidied := func(e Event) bool {
		u, ok := e.Body.(MessageUpserted)
		return ok && u.Message.Kind == org.MessageSystem
	}
	r, evs := f.send(t, "hi", tidied)
	sys := evs[len(evs)-1].Body.(MessageUpserted).Message
	if sys.Content != "Dana tidied their notes: 200 became 2" || sys.RunID != r.ID || sys.AuthorBotID != f.bot.ID {
		t.Errorf("system message = %+v", sys)
	}
	deleted, added := 0, 0
	for _, e := range evs {
		if n, ok := e.Body.(NoteChanged); ok {
			if n.Deleted {
				deleted++
			} else {
				added++
			}
		}
	}
	if deleted != memory.Cap || added != 2 {
		t.Errorf("note events: %d deleted, %d added; want %d and 2", deleted, added, memory.Cap)
	}
	if got := l.runs()[r.ID]; got != 2 {
		t.Errorf("the ledger holds %d requests for the run, want the run's and the tidy's", got)
	}
}

func TestAfter_AResumedRunIsMetered(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(llm.StreamEvent{Usage: &llm.Usage{PromptTokens: 10}}, toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		reply("Done."),
	)
	l := withLedger(t, f)
	_, it := f.park(t, "tidy up")
	resumed := f.decide(t, it, inbox.AllowOnce, "")
	if got := l.runs()[resumed.ID]; got != 1 {
		t.Errorf("the ledger holds %d requests for the resumed run, want 1", got)
	}
}
