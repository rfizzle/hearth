package files

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

// deleter is a bot's folder where delete is granted in ask mode, as a new
// bot has it, with read and write allowed, and a folder outside it. The
// folder is left unresolved: on macOS t.TempDir is under /var, a link to
// /private/var.
type deleter struct {
	folder, outside string
	d               tools.Dispatcher
}

func newDeleter(t *testing.T) deleter {
	t.Helper()
	base := t.TempDir()
	_, grants, _ := org.NewBot("Writer", "", "", org.TierFast, base)
	dl := deleter{folder: filepath.Join(base, "Writer"), outside: filepath.Join(base, "elsewhere")}
	for _, dir := range []string{dl.folder, filepath.Join(dl.folder, "old"), dl.outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{filepath.Join(dl.folder, "brief.md"), filepath.Join(dl.folder, "old", "brief.md"), filepath.Join(dl.outside, "x.md")} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for link, to := range map[string]string{"link.md": filepath.Join(dl.folder, "brief.md"), "out": dl.outside, "out.md": filepath.Join(dl.outside, "x.md")} {
		if err := os.Symlink(to, filepath.Join(dl.folder, link)); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := tools.NewRegistry(Definitions()...)
	if err != nil {
		t.Fatal(err)
	}
	dl.d = tools.Dispatcher{Defs: reg, Grants: grants, Root: dl.folder}
	return dl
}

func TestDelete(t *testing.T) {
	cases := []struct {
		name, path string
		before     string // refused by the dispatcher before the gate, containing this
		refusal    string // refused once allowed, containing this
		gone       string // the file the delete removes, relative to the folder
		effect     string
	}{
		{name: "a file in the bot's folder", path: "brief.md", gone: "brief.md", effect: "Delete brief.md from Writer's folder"},
		{name: "a file in a folder inside it", path: "old/brief.md", gone: "old/brief.md", effect: "Delete old/brief.md from Writer's folder"},
		{name: "a path through .. that stays inside", path: "old/../brief.md", gone: "brief.md", effect: "Delete brief.md from Writer's folder"},
		{name: "a folder", path: "old", refusal: "old is a folder, not a file", effect: "Delete old from Writer's folder"},
		{name: "the folder itself", path: ".", refusal: "is not a file inside the folder"},
		{name: "a link to a file inside", path: "link.md", refusal: "link.md is a link", effect: "Delete brief.md from Writer's folder"},
		{name: "a link out of the folder", path: "out/x.md", before: "outside the folders"},
		{name: "a link to a file outside", path: "out.md", before: "outside the folders"},
		{name: "an absolute path outside", path: "/etc/hosts", before: "outside the folders"},
		{name: "a file that is not there", path: "nope.md", refusal: "no such file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dl := newDeleter(t)
			args, _ := json.Marshal(map[string]string{"path": tc.path})
			c, err := dl.d.Check("files.delete", args)
			if tc.before != "" {
				if !errors.Is(err, tools.ErrRefused) || !strings.Contains(err.Error(), tc.before) || dl.d.Gated("files.delete", args) {
					t.Fatalf("Check = %v, want a refusal containing %q and no gate", err, tc.before)
				}
				if _, err := os.Stat(filepath.Join(dl.outside, "x.md")); err != nil {
					t.Fatal("the file outside is gone")
				}
				return
			}
			if err != nil || c.Verdict != tools.Gated {
				t.Fatalf("Check = %+v, %v, want it to wait for the owner", c.Verdict, err)
			}
			if tc.effect != "" && c.Describe() != tc.effect {
				t.Errorf("card = %q, want %q", c.Describe(), tc.effect)
			}
			if _, err := dl.d.Execute(context.Background(), "files.delete", args); err == nil || !strings.Contains(err.Error(), "approval") {
				t.Fatalf("Execute before a decision = %v, want it refused", err)
			}
			res, err := dl.d.ExecuteApproved(context.Background(), "files.delete", args)
			if tc.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("ExecuteApproved = %+v, %v, want a refusal containing %q", res, err, tc.refusal)
				}
				for _, kept := range []string{"brief.md", "old/brief.md", "link.md"} {
					if _, err := os.Lstat(filepath.Join(dl.folder, kept)); err != nil {
						t.Errorf("%s is gone after a refused delete", kept)
					}
				}
				return
			}
			if err != nil || res.Text != "Deleted "+tc.gone+"." || strings.Contains(res.Text, dl.folder) {
				t.Fatalf("ExecuteApproved = %+v, %v", res, err)
			}
			if _, err := os.Lstat(filepath.Join(dl.folder, tc.gone)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("%s is still there: %v", tc.gone, err)
			}
		})
	}
}

func TestDelete_ANamedPipeIsNotDeleted(t *testing.T) {
	b := newBot(t, org.FilesDelete)
	b.fifo(t, "pipe")
	if _, err := b.d.ExecuteApproved(context.Background(), "files.delete", json.RawMessage(`{"path":"pipe"}`)); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("ExecuteApproved = %v, want it refused", err)
	}
}

func TestTarget(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a, b := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	for _, f := range []string{a, b} {
		if err := os.WriteFile(f, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(a, filepath.Join(dir, "l.md")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		call    tools.Call
		rel     string
		refusal string
	}{
		{"the file the call named", tools.Call{Folder: dir, Path: a, Requested: a}, "a.md", ""},
		{"a link at the end of the path", tools.Call{Folder: dir, Path: a, Requested: filepath.Join(dir, "l.md")}, "", "l.md is a link"},
		{"a path that now leads elsewhere", tools.Call{Folder: dir, Path: b, Requested: a}, "", "no longer leads to the file that was checked"},
		{"the folder", tools.Call{Folder: dir, Path: dir, Requested: dir}, "", "not a file inside the folder"},
		{"a path outside the folder", tools.Call{Folder: filepath.Join(dir, "sub"), Path: a, Requested: a}, "", "not a file inside the folder"},
		{"a file gone", tools.Call{Folder: dir, Path: filepath.Join(dir, "c.md"), Requested: filepath.Join(dir, "c.md")}, "", "no such file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel, _, err := target(tc.call)
			if tc.refusal != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("target = %q, %v, want a refusal containing %q", rel, err, tc.refusal)
				}
				return
			}
			if err != nil || rel != tc.rel {
				t.Fatalf("target = %q, %v, want %q", rel, err, tc.rel)
			}
		})
	}
}

func TestDescribeDelete(t *testing.T) {
	cases := []struct{ args, want string }{
		{`{"path":"Dana/brief.md","folder":"Dana"}`, "Delete brief.md from Dana's folder"},
		{`{"path":"Dana/old/brief.md","folder":"Dana"}`, "Delete old/brief.md from Dana's folder"},
		{`{"path":"brief.md"}`, "Delete brief.md"},
		{`{"path":"notes/brief.md"}`, "Delete notes/brief.md"},
		{`{"path":"x.md","folder":"Dana"}`, "Delete x.md"},
		{`nope`, "Delete "},
	}
	for _, tc := range cases {
		if got := describeDelete(json.RawMessage(tc.args)); got != tc.want {
			t.Errorf("describeDelete(%s) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestDeleteDefinition(t *testing.T) {
	d := deleteDefinition()
	if d.Name != "files.delete" || d.Capability != org.FilesDelete || d.Tier != tools.TierAct || !d.NoStanding || d.Standing() {
		t.Fatalf("definition = %+v", d)
	}
	for _, want := range []string{"only a file the owner asked", "name the file in your reply"} {
		if !strings.Contains(d.Description, want) {
			t.Errorf("description is missing %q", want)
		}
	}
	reg, err := tools.NewRegistry(Definitions()...)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	withoutDelete := tools.Dispatcher{Defs: reg, Root: folder, Grants: []org.Grant{
		{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
		{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
	}}
	for _, def := range withoutDelete.Tools() {
		if def.Name == "files.delete" {
			t.Fatal("a dispatcher without the delete grant offers files.delete")
		}
	}
	if _, err := withoutDelete.Check("files.delete", json.RawMessage(`{"path":"a.md"}`)); !errors.Is(err, tools.ErrRefused) {
		t.Fatalf("Check without the grant = %v, want a refusal", err)
	}
}
