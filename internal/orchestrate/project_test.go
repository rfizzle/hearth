package orchestrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/workspace"
)

// projectFixture is a fixture with an active project, whose conversation the
// fixture's bot answers as the coordinator when asked to.
func projectFixture(t *testing.T, steps ...step) (*fixture, project.Project) {
	t.Helper()
	f := newFixture(t, steps...)
	ctx := context.Background()
	o, err := f.db.Org(ctx)
	if err != nil {
		t.Fatal(err)
	}
	brief := project.Brief{Goal: "Ship the launch page", Context: "Small team", DoneLooksLike: "A page is live", DoneWhen: "The page loads."}
	p, conv, err := project.New("Launch", brief, "2026-12-01", o.WorkspacePath)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = f.db.CreateProject(ctx, p, conv); err != nil {
		t.Fatal(err)
	}
	return f, p
}

func (f *fixture) coordinate() {
	f.svc.d.Coordinator = func(context.Context) (org.Bot, bool) { return f.bot, true }
}

func TestProjectConversation(t *testing.T) {
	t.Run("a message runs the coordinator with the brief, and a file lands in the project folder", func(t *testing.T) {
		f, p := projectFixture(t,
			events(toolCall("c1", "files.write", `{"path":"report.md","content":"# Report\n"}`)),
			events(llm.StreamEvent{Token: "Done."}, llm.StreamEvent{Done: true}),
		)
		f.coordinate()
		ch, cancel := f.bus.Subscribe(0)
		defer cancel()
		if _, _, err := f.svc.Send(context.Background(), p.ConversationID, "Start.", nil); err != nil {
			t.Fatal(err)
		}
		until(t, ch, ended)

		o, _ := f.db.Org(context.Background())
		path := filepath.Join(o.WorkspacePath, "Projects", "Launch", "report.md")
		if b, err := os.ReadFile(path); err != nil || string(b) != "# Report\n" {
			t.Fatalf("report.md = %q, %v", b, err)
		}
		msgs, _, err := f.db.Messages(context.Background(), p.ConversationID, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		var files []org.FileRef
		for _, m := range msgs {
			files = append(files, m.Files...)
		}
		if len(files) != 1 || files[0].Path != path {
			t.Errorf("file refs = %+v, want %s", files, path)
		}
		f.script.mu.Lock()
		defer f.script.mu.Unlock()
		sent := f.script.seen[0][0].Content
		for _, want := range []string{`the project "Launch"`, "Goal: Ship the launch page", "Done when: The page loads.", "Target date: 2026-12-01", filepath.Dir(path)} {
			if !strings.Contains(sent, want) {
				t.Errorf("system text is missing %q:\n%s", want, sent)
			}
		}
	})

	t.Run("with no coordinator the message is refused and nothing is stored", func(t *testing.T) {
		f, p := projectFixture(t)
		_, _, err := f.svc.Send(context.Background(), p.ConversationID, "Start.", nil)
		if !errors.Is(err, ErrNoCoordinator) {
			t.Fatalf("Send = %v, want ErrNoCoordinator", err)
		}
		msgs, _, err := f.db.Messages(context.Background(), p.ConversationID, 0, 0)
		if err != nil || len(msgs) != 0 {
			t.Errorf("messages = %+v, %v, want none", msgs, err)
		}
		if _, ok := f.svc.Coordinator(context.Background()); ok {
			t.Error("Coordinator answers with none wired")
		}
	})

	t.Run("a direct conversation still goes to its bot, with no project text", func(t *testing.T) {
		f := newFixture(t, events(llm.StreamEvent{Token: "Hi."}, llm.StreamEvent{Done: true}))
		f.coordinate()
		f.send(t, "Hello.", ended)
		f.script.mu.Lock()
		defer f.script.mu.Unlock()
		if sent := f.script.seen[0][0].Content; strings.Contains(sent, "## The project") {
			t.Errorf("a direct conversation has the project section:\n%s", sent)
		}
	})

	t.Run("an unknown conversation is not found", func(t *testing.T) {
		f := newFixture(t)
		if _, _, err := f.svc.Send(context.Background(), "nope", "x", nil); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Send = %v, want ErrNotFound", err)
		}
	})
}

func TestProjectGrants(t *testing.T) {
	f, p := projectFixture(t)
	ctx := context.Background()
	o, _ := f.db.Org(ctx)
	if err := workspace.Ensure(o.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	disp, err := f.svc.dispatcher(ctx, f.bot, p.ConversationID, "", &p)
	if err != nil {
		t.Fatal(err)
	}
	if disp.Root != p.FolderPath {
		t.Errorf("root = %s, want %s", disp.Root, p.FolderPath)
	}
	has := func(grants []org.Grant, c org.Capability) bool {
		for _, g := range grants {
			if g.Capability == c && g.Folder == p.FolderPath {
				return true
			}
		}
		return false
	}
	if !has(disp.Grants, org.FilesRead) || !has(disp.Grants, org.FilesWrite) {
		t.Errorf("grants = %+v, want read and write on the project folder", disp.Grants)
	}
	if has(disp.Grants, org.FilesDelete) {
		t.Error("the project folder is deletable")
	}
	if err := os.MkdirAll(p.FolderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := disp.Check("files.delete", []byte(`{"path":"report.md"}`)); err == nil {
		t.Error("files.delete in the project folder is allowed")
	}
	stored, err := f.db.GrantsFor(ctx, f.bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range stored {
		if strings.HasPrefix(g.Folder, p.FolderPath) {
			t.Errorf("stored grant %+v is on the project folder", g)
		}
	}
	if st, err := f.svc.status(ctx, p.ID); err != nil || st.Light != project.LightNone {
		t.Errorf("status = %+v, %v, want none", st, err)
	}
}

// A run is offered deliverables.write once the office's figures are wired,
// and this means the project of the conversation: refused in words in a
// direct one, resolved in a project's.
func TestDeliverablesBindThisProject(t *testing.T) {
	f, p := projectFixture(t)
	ctx := context.Background()
	o, _ := f.db.Org(ctx)
	if err := workspace.Ensure(o.WorkspacePath); err != nil {
		t.Fatal(err)
	}
	if offers(t, f, "deliverables.write") {
		t.Error("deliverables.write is offered with no measures wired")
	}
	if err := os.MkdirAll(p.FolderPath, 0o755); err != nil {
		t.Fatal(err)
	}
	f.svc.d.Measures = measure.Deps{}
	args := []byte(`{"path":"d","spec":{"version":1,"kind":"report","title":"T","sections":[{"widgets":[{"kind":"progress","label":"Tasks","measure":"project.tasks/this"}]}]}}`)
	dm, err := f.svc.dispatcher(ctx, f.bot, f.bot.DMConversationID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dm.Check("deliverables.write", args); err == nil || !strings.Contains(err.Error(), "not in a project") {
		t.Errorf("this in a direct conversation: %v", err)
	}
	in, err := f.svc.dispatcher(ctx, f.bot, p.ConversationID, "", &p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.Check("deliverables.write", args); err != nil {
		t.Errorf("this in the project's conversation: %v", err)
	}
}
