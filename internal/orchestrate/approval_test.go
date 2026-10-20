package orchestrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// parkedFixture is a fixture with the given files in the bot's folder, which
// a new bot may delete only by asking.
func parkedFixture(t *testing.T, files []string, steps ...step) *fixture {
	t.Helper()
	f := newFixture(t, steps...)
	if err := os.MkdirAll(f.bot.FolderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(f.bot.FolderPath, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func runIs(id string, s org.RunState) func(Event) bool {
	return func(e Event) bool {
		rc, ok := e.Body.(RunChanged)
		return ok && (id == "" || rc.Run.ID == id) && rc.Run.State == s
	}
}

func runEnded(id string) func(Event) bool {
	return func(e Event) bool { return ended(e) && e.Body.(RunChanged).Run.ID == id }
}

// park sends text and waits for the run to wait on the owner, returning the
// run and the open item.
func (f *fixture) park(t *testing.T, text string) (org.Run, inbox.Item) {
	t.Helper()
	r, _ := f.send(t, text, runIs("", org.RunWaitingOwner))
	it, open, err := f.db.OpenItemFor(context.Background(), f.bot.DMConversationID)
	if err != nil || !open {
		t.Fatalf("open item = %v, %v", open, err)
	}
	return r, it
}

// decide decides the item and waits for the resumed run to end.
func (f *fixture) decide(t *testing.T, it inbox.Item, d inbox.Decision, reason string) org.Run {
	t.Helper()
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	closed, r, err := f.svc.Decide(context.Background(), it.ID, d, reason)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if closed.State != inbox.Closed || closed.Decision != d {
		t.Fatalf("item = %+v, want closed with %d", closed, d)
	}
	until(t, ch, runEnded(r.ID))
	return f.storedRun(t, r.ID)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// sentTail is the resumed request's messages after the owner's first one.
func sentTail(t *testing.T, msgs []llm.Message) []llm.Message {
	t.Helper()
	for i, m := range msgs {
		if m.Role == llm.RoleUser {
			return msgs[i+1:]
		}
	}
	t.Fatalf("no owner message in %+v", msgs)
	return nil
}

func TestApproval_ParkAndAllowOnce(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.write", `{"path":"notes.md","content":"n"}`)),
		events(llm.StreamEvent{Token: "Tidying up."}, toolCall("c2", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Done."}, llm.StreamEvent{Done: true}),
	)
	parkedRun, it := f.park(t, "tidy up")
	brief := filepath.Join(f.bot.FolderPath, "brief.md")

	if got := f.storedRun(t, parkedRun.ID); got.State != org.RunWaitingOwner || !got.EndedAt.IsZero() {
		t.Fatalf("run = %+v, want waiting with no end", got)
	}
	msgs := f.messages(t)
	req := msgs[len(msgs)-1]
	if req.Kind != org.MessageApprovalRequest || req.Content != "Delete brief.md from Dana's folder" || req.RunID != parkedRun.ID {
		t.Fatalf("last message = %+v, want the request in the describer's words", req)
	}
	if it.Kind != inbox.Approval || it.Effect != req.Content || it.MessageID != req.ID || it.Call.ID != "c2" {
		t.Fatalf("item = %+v", it)
	}
	for _, m := range msgs {
		if m.Kind == org.MessageToolResult && m.ToolCallID == "c2" || strings.Contains(m.Content, "waiting for the owner") {
			t.Fatalf("the parked call has a result before a decision: %+v", m)
		}
	}
	if !exists(brief) {
		t.Fatal("brief.md was deleted before a decision")
	}
	if _, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "hello?", nil); !errors.Is(err, ErrWaiting) {
		t.Fatalf("Send while waiting = %v, want ErrWaiting", err)
	}
	if _, err := f.svc.Retry(context.Background(), f.bot.DMConversationID); !errors.Is(err, ErrWaiting) {
		t.Fatalf("Retry while waiting = %v, want ErrWaiting", err)
	}

	resumed := f.decide(t, it, inbox.AllowOnce, "")
	if exists(brief) {
		t.Fatal("brief.md is still there after Allow once")
	}
	if resumed.State != org.RunDone || f.storedRun(t, parkedRun.ID).State != org.RunDone {
		t.Fatalf("resumed run %+v, parked run %+v, want both done", resumed, f.storedRun(t, parkedRun.ID))
	}
	var decision, result *org.Message
	for _, m := range f.messages(t) {
		switch {
		case m.Kind == org.MessageApprovalDecision:
			decision = &m
		case m.Kind == org.MessageToolResult && m.ToolCallID == "c2":
			result = &m
		}
	}
	if decision == nil || decision.Content != "Allowed once" || decision.AuthorBotID != "" {
		t.Fatalf("decision = %+v", decision)
	}
	if result == nil || result.Content != "Deleted brief.md." || result.RunID != parkedRun.ID {
		t.Fatalf("result = %+v", result)
	}
	if _, open, _ := f.db.OpenItemFor(context.Background(), f.bot.DMConversationID); open {
		t.Fatal("the item is still open")
	}

	tail := sentTail(t, f.script.seen[2])
	if len(tail) != 3 {
		t.Fatalf("resumed request after the owner's message = %+v, want the round and the note", tail)
	}
	bot, tool, note := tail[0], tail[1], tail[2]
	if bot.Role != llm.RoleAssistant || bot.Content != "Tidying up." || len(bot.ToolCalls) != 1 || bot.ToolCalls[0].ID != "c2" {
		t.Errorf("bot message = %+v, want the parked round's text and call", bot)
	}
	if tool.Role != llm.RoleTool || tool.ToolCallID != "c2" || tool.Content != "Deleted brief.md." {
		t.Errorf("tool message = %+v, want the real result", tool)
	}
	if note.Role != llm.RoleUser || note.Content != assembly.Note {
		t.Errorf("prompt = %+v, want the note", note)
	}
}

func TestApproval_DenyGivesTheReason(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Understood."}, llm.StreamEvent{Done: true}),
	)
	_, it := f.park(t, "tidy up")
	f.decide(t, it, inbox.Deny, "not that one")
	if !exists(filepath.Join(f.bot.FolderPath, "brief.md")) {
		t.Fatal("a denied delete removed the file")
	}
	tail := sentTail(t, f.script.seen[1])
	if tail[1].Role != llm.RoleTool || !strings.Contains(tail[1].Content, "Reason: not that one.") {
		t.Fatalf("resumed tool message = %+v, want the denial", tail[1])
	}
	msgs := f.messages(t)
	var said string
	for _, m := range msgs {
		if m.Kind == org.MessageApprovalDecision {
			said = m.Content
		}
	}
	if said != "Declined: not that one" {
		t.Errorf("decision message = %q", said)
	}
}

func TestApproval_AlwaysAllow(t *testing.T) {
	t.Run("a tool with no standing is refused and the item stays open", func(t *testing.T) {
		f := parkedFixture(t, []string{"brief.md"}, events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)))
		r, it := f.park(t, "tidy up")
		if _, _, err := f.svc.Decide(context.Background(), it.ID, inbox.AlwaysAllow, ""); !errors.Is(err, inbox.ErrNoStanding) {
			t.Fatalf("Decide = %v, want ErrNoStanding", err)
		}
		if _, open, _ := f.db.OpenItemFor(context.Background(), f.bot.DMConversationID); !open || f.storedRun(t, r.ID).State != org.RunWaitingOwner {
			t.Fatal("a refused decision closed the item or ended the run")
		}
		if !exists(filepath.Join(f.bot.FolderPath, "brief.md")) {
			t.Fatal("a refused decision ran the call")
		}
		if _, _, err := f.svc.Send(context.Background(), f.bot.DMConversationID, "hm", nil); !errors.Is(err, ErrWaiting) {
			t.Fatalf("Send = %v, want ErrWaiting: the conversation is still held by the item", err)
		}
	})

	t.Run("a write to an ask-me folder parks, then the grant is raised", func(t *testing.T) {
		shared := filepath.Join(t.TempDir(), "Shared")
		if err := os.MkdirAll(shared, 0o755); err != nil {
			t.Fatal(err)
		}
		args := `{"path":"` + filepath.Join(shared, "a.md") + `","content":"a"}`
		f := parkedFixture(t, nil,
			events(toolCall("c1", "files.write", args)),
			events(llm.StreamEvent{Token: "Saved."}, llm.StreamEvent{Done: true}),
			events(toolCall("c2", "files.write", strings.Replace(args, "a.md", "b.md", 1))),
			events(llm.StreamEvent{Token: "Saved again."}, llm.StreamEvent{Done: true}),
		)
		ctx := context.Background()
		grants := []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: shared}}
		if err := f.db.ReplaceGrants(ctx, f.bot.ID, grants); err != nil {
			t.Fatal(err)
		}
		_, it := f.park(t, "save it")
		if it.Effect != "Write Shared/a.md (1 byte)" {
			t.Fatalf("effect = %q, want the describer's words", it.Effect)
		}
		f.decide(t, it, inbox.AlwaysAllow, "")
		if !exists(filepath.Join(shared, "a.md")) {
			t.Fatal("the approved write did not run")
		}
		got, err := f.db.GrantsFor(ctx, f.bot.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(org.Folders(got, org.FilesWrite, org.ModeAllowed), shared) {
			t.Fatalf("grants = %+v, want the shared folder allowed", got)
		}
		r, _ := f.send(t, "save another", ended)
		if f.storedRun(t, r.ID).State != org.RunDone || !exists(filepath.Join(shared, "b.md")) {
			t.Fatal("the next write to the folder waited again")
		}
	})
}

func TestApproval_AllowForTaskCoversTheNextCall(t *testing.T) {
	f := parkedFixture(t, []string{"a.md", "b.md"},
		events(toolCall("c1", "files.delete", `{"path":"a.md"}`)),
		events(toolCall("c2", "files.delete", `{"path":"b.md"}`)),
		events(llm.StreamEvent{Token: "Both gone."}, llm.StreamEvent{Done: true}),
	)
	_, it := f.park(t, "delete both")
	resumed := f.decide(t, it, inbox.AllowForTask, "")
	if resumed.State != org.RunDone {
		t.Fatalf("resumed run = %+v, want done without waiting again", resumed)
	}
	if exists(filepath.Join(f.bot.FolderPath, "a.md")) || exists(filepath.Join(f.bot.FolderPath, "b.md")) {
		t.Fatal("a file survived")
	}
	all, err := f.db.InboxItems(context.Background(), false)
	if err != nil || len(all) != 1 {
		t.Fatalf("items = %+v, %v, want the one", all, err)
	}
	allowances, err := f.db.Allowances(context.Background(), f.bot.DMConversationID)
	if err != nil || len(allowances) != 1 || allowances[0].Capability != org.FilesDelete {
		t.Fatalf("allowances = %+v, %v", allowances, err)
	}
}

func TestApproval_CancelClosesTheItem(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Hi."}, llm.StreamEvent{Done: true}),
	)
	r, it := f.park(t, "tidy up")
	if !f.svc.Cancel(r.ID) {
		t.Fatal("Cancel did not stop the waiting run")
	}
	got, err := f.db.InboxItem(context.Background(), it.ID)
	if err != nil || got.State != inbox.Closed || got.Outcome != "Cancelled" {
		t.Fatalf("item = %+v, %v, want closed as cancelled", got, err)
	}
	if run := f.storedRun(t, r.ID); run.State != org.RunCancelled || run.EndedAt.IsZero() {
		t.Fatalf("run = %+v, want cancelled", run)
	}
	if f.svc.Cancel(r.ID) {
		t.Fatal("Cancel stopped a run twice")
	}
	if _, _, err := f.svc.Decide(context.Background(), it.ID, inbox.AllowOnce, ""); !errors.Is(err, ErrClosed) {
		t.Fatalf("Decide after Cancel = %v, want ErrClosed", err)
	}
	f.send(t, "hello", ended)
}

func TestApproval_RelaunchLeavesTheWaitAsItWas(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Gone."}, llm.StreamEvent{Done: true}),
	)
	r, it := f.park(t, "tidy up")
	svc, err := Open(context.Background(), Deps{Store: f.db, Streams: f.svc.d.Streams, Bus: f.bus})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	if got := f.storedRun(t, r.ID); got.State != org.RunWaitingOwner {
		t.Fatalf("run after relaunch = %+v, want still waiting", got)
	}
	if got, err := f.db.InboxItem(context.Background(), it.ID); err != nil || got.State != inbox.Open {
		t.Fatalf("item after relaunch = %+v, %v, want still open", got, err)
	}
	if resumed := f.decide(t, it, inbox.AllowOnce, ""); resumed.State != org.RunDone {
		t.Fatalf("resumed run = %+v", resumed)
	}
	if exists(filepath.Join(f.bot.FolderPath, "brief.md")) {
		t.Fatal("the decision after relaunch did not run the call")
	}
}

func TestApproval_Refusals(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"}, events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)))
	_, it := f.park(t, "tidy up")
	ctx := context.Background()
	if _, _, err := f.svc.Decide(ctx, it.ID, 9, ""); !errors.Is(err, ErrMismatch) {
		t.Errorf("an unknown decision = %v, want ErrMismatch", err)
	}
	if _, _, err := f.svc.Answer(ctx, it.ID, "yes", nil); !errors.Is(err, ErrMismatch) {
		t.Errorf("an answer to an approval = %v, want ErrMismatch", err)
	}
	if _, err := f.svc.Acknowledge(ctx, it.ID); !errors.Is(err, ErrMismatch) {
		t.Errorf("acknowledging an approval = %v, want ErrMismatch", err)
	}
	if _, _, err := f.svc.Decide(ctx, "nope", inbox.AllowOnce, ""); err == nil {
		t.Error("a decision on no item was accepted")
	}
	if _, _, err := f.svc.Answer(ctx, it.ID, "  ", nil); !errors.Is(err, ErrMismatch) {
		t.Errorf("an empty answer = %v, want ErrMismatch", err)
	}
}

func TestApproval_AnItemWhoseRunEndedHoldsNothing(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Hi."}, llm.StreamEvent{Done: true}),
	)
	r, it := f.park(t, "tidy up")
	// The core stopped between storing the item and the run.
	stuck := f.storedRun(t, r.ID)
	stuck.State = org.RunFailed
	if err := f.db.UpdateRun(context.Background(), stuck); err != nil {
		t.Fatal(err)
	}
	f.send(t, "hello", ended)
	if got, _ := f.db.InboxItem(context.Background(), it.ID); got.State != inbox.Closed {
		t.Fatalf("item = %+v, want closed as stale", got)
	}
}

func TestApproval_TheItemIsAnnouncedOnceTheConversationIsFree(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"},
		events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)),
		events(llm.StreamEvent{Token: "Gone."}, llm.StreamEvent{Done: true}),
	)
	_, evs := f.send(t, "tidy up", func(e Event) bool { _, ok := e.Body.(InboxChanged); return ok })
	it := evs[len(evs)-1].Body.(InboxChanged).Item
	ch, cancel := f.bus.Subscribe(0)
	defer cancel()
	_, r, err := f.svc.Decide(context.Background(), it.ID, inbox.AllowOnce, "")
	if err != nil {
		t.Fatalf("Decide on the item's event = %v", err)
	}
	until(t, ch, runEnded(r.ID))
}

func TestApproval_AlwaysAllowRaisesOnlyTheFolderAsked(t *testing.T) {
	outer := filepath.Join(t.TempDir(), "A")
	inner := filepath.Join(outer, "B")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	f := parkedFixture(t, nil,
		events(toolCall("c1", "files.write", `{"path":"`+filepath.Join(inner, "x.md")+`","content":"x"}`)),
		events(llm.StreamEvent{Token: "Saved."}, llm.StreamEvent{Done: true}),
	)
	ctx := context.Background()
	grants := []org.Grant{
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: inner},
		{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: outer},
	}
	if err := f.db.ReplaceGrants(ctx, f.bot.ID, grants); err != nil {
		t.Fatal(err)
	}
	_, it := f.park(t, "save it")
	f.decide(t, it, inbox.AlwaysAllow, "")
	got, err := f.db.GrantsFor(ctx, f.bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	allowed := org.Folders(got, org.FilesWrite, org.ModeAllowed)
	if !slices.Contains(allowed, inner) || slices.Contains(allowed, outer) {
		t.Fatalf("allowed folders = %v, want %s and not %s", allowed, inner, outer)
	}
}

func TestApproval_TheCardNamesTheFolderTheCallLandsIn(t *testing.T) {
	cases := []struct{ name, path string }{
		{"dot-dot out of the bot's folder", "notes/../../Other/tax.md"},
		{"a link out of the bot's folder", "link/tax.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := parkedFixture(t, nil,
				events(toolCall("c1", "files.write", `{"path":"`+tc.path+`","content":"x"}`)),
				events(llm.StreamEvent{Token: "Saved."}, llm.StreamEvent{Done: true}),
			)
			ctx := context.Background()
			other := filepath.Join(filepath.Dir(f.bot.FolderPath), "Other")
			if err := os.MkdirAll(filepath.Join(f.bot.FolderPath, "notes"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(other, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(other, filepath.Join(f.bot.FolderPath, "link")); err != nil {
				t.Fatal(err)
			}
			grants := []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: other}}
			if err := f.db.ReplaceGrants(ctx, f.bot.ID, grants); err != nil {
				t.Fatal(err)
			}
			_, it := f.park(t, "save it")
			msgs := f.messages(t)
			if want := "Write Other/tax.md (1 byte)"; it.Effect != want || msgs[len(msgs)-1].Content != want {
				t.Fatalf("card = %q, item = %q, want %q", msgs[len(msgs)-1].Content, it.Effect, want)
			}
			f.decide(t, it, inbox.AlwaysAllow, "")
			var said string
			for _, m := range f.messages(t) {
				if m.Kind == org.MessageApprovalDecision {
					said = m.Content
				}
			}
			if said != "Always allowed for Dana in Other" {
				t.Fatalf("decision = %q, want it to name the folder", said)
			}
		})
	}
}

func TestApproval_AFailureAfterTheCallRanEndsTheRunFailed(t *testing.T) {
	f := parkedFixture(t, []string{"brief.md"}, events(toolCall("c1", "files.delete", `{"path":"brief.md"}`)))
	r, it := f.park(t, "tidy up")
	f.svc.fault = func() error { return errors.New("disk full") }
	if _, _, err := f.svc.Decide(context.Background(), it.ID, inbox.AllowOnce, ""); err == nil {
		t.Fatal("Decide reported success")
	}
	got := f.storedRun(t, r.ID)
	if got.State != org.RunFailed || got.Failure != abandonReason || got.EndedAt.IsZero() {
		t.Fatalf("run = %+v, want failed with the reason", got)
	}
	msgs := f.messages(t)
	if last := msgs[len(msgs)-1]; last.Kind != org.MessageSystem || last.RunID != r.ID {
		t.Fatalf("last message = %+v, want the reason in the thread", last)
	}
	f.svc.fault = nil
	f.send(t, "again", ended) // the conversation is free again
}

func TestApproval_TheCallRowNamesTheFileTheCallLandsOn(t *testing.T) {
	f := parkedFixture(t, []string{"x.md"}, events(toolCall("c1", "files.delete", `{"path":"notes/../x.md"}`)))
	if err := os.MkdirAll(filepath.Join(f.bot.FolderPath, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, it := f.park(t, "tidy up")
	var row string
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageToolCall && len(m.ToolCalls) == 1 && m.ToolCalls[0].ID == "c1" {
			row = m.ToolCalls[0].Summary
		}
	}
	if want := "Delete x.md from Dana's folder"; row != want || it.Effect != want {
		t.Fatalf("row = %q, card = %q, want both %q", row, it.Effect, want)
	}
}
