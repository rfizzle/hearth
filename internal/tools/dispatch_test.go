package tools

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

// recorder is a definition's executor that remembers the last call it ran.
type recorder struct{ last *Call }

func (r *recorder) def(name string, capability org.Capability, tier Tier) Definition {
	return Definition{
		Name:       name,
		Capability: capability,
		Tier:       tier,
		Execute: func(_ context.Context, call Call) (Result, error) {
			r.last = &call
			return Result{Text: "ran " + name}, nil
		},
		Describe: func(json.RawMessage) string { return name },
	}
}

func registry(t *testing.T, r *recorder) *Registry {
	t.Helper()
	reg, err := NewRegistry(
		r.def("files.read", org.FilesRead, TierRead),
		r.def("files.write", org.FilesWrite, TierRead),
		r.def("web.fetch", org.WebFetch, TierRead),
		r.def("calendar.write", "calendar.write", TierAct),
	)
	if err != nil {
		t.Fatal(err)
	}
	return reg
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func TestDispatcher_Execute(t *testing.T) {
	// base is deliberately not resolved: on macOS t.TempDir is under /var,
	// which is a symlink to /private/var, the trap grants fall into.
	base := t.TempDir()
	real, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	bot := filepath.Join(base, "Hearth", "Writer")
	outside := filepath.Join(base, "elsewhere")
	mkdir(t, bot)
	mkdir(t, outside)
	symlink(t, outside, filepath.Join(bot, "out"))
	// A granted folder reached through a symlinked parent.
	symlink(t, filepath.Join(base, "Hearth"), filepath.Join(base, "alias"))
	aliased := filepath.Join(base, "alias", "Writer")

	readWrite := func(folder string) []org.Grant {
		return []org.Grant{
			{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.WebFetch, Mode: org.ModeAllowed},
		}
	}
	readOnly := []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: bot}}
	askWrite := []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAsk, Folder: bot}}
	inBot := filepath.Join(real, "Hearth", "Writer")

	cases := []struct {
		name     string
		grants   []org.Grant
		root     string
		tool     string
		args     string
		wantPath string // the resolved path the executor sees; "" when refused
		refusal  string // a fragment of the refusal; "" when it runs
	}{
		{"a relative path inside", readWrite(bot), bot, "files.write", `{"path":"brief.md"}`, filepath.Join(inBot, "brief.md"), ""},
		{"a new subfolder inside", readWrite(bot), bot, "files.write", `{"path":"notes/a.md"}`, filepath.Join(inBot, "notes", "a.md"), ""},
		{"an absolute path inside", readWrite(bot), bot, "files.read", `{"path":"` + filepath.Join(bot, "x.md") + `"}`, filepath.Join(inBot, "x.md"), ""},
		{"a granted folder through a symlinked parent", readWrite(aliased), aliased, "files.write", `{"path":"brief.md"}`, filepath.Join(inBot, "brief.md"), ""},
		{"a bot without files.write", readOnly, bot, "files.write", `{"path":"brief.md"}`, "", "not allowed"},
		{"a grant in ask mode", askWrite, bot, "files.write", `{"path":"brief.md"}`, "", "not allowed"},
		{"a path through a symlink out", readWrite(bot), bot, "files.read", `{"path":"out/secret.txt"}`, "", "outside the folders"},
		{"dot-dot out of the folder", readWrite(bot), bot, "files.write", `{"path":"../../elsewhere/x.md"}`, "", "outside the folders"},
		{"an absolute path outside", readWrite(bot), bot, "files.read", `{"path":"` + filepath.Join(outside, "x") + `"}`, "", "outside the folders"},
		{"a relative path with no root", readWrite(bot), "", "files.read", `{"path":"x.md"}`, "", "no folder"},
		{"a relative root", readWrite(bot), "Hearth/Writer", "files.write", `{"path":"x.md"}`, "", "not an absolute path"},
		{"a relative root with an absolute path", readWrite(bot), "Hearth/Writer", "files.read", `{"path":"` + filepath.Join(bot, "x.md") + `"}`, "", "not an absolute path"},
		{"a granted folder that does not exist", readWrite(filepath.Join(base, "gone")), bot, "files.read", `{"path":"x.md"}`, "", "outside the folders"},
		{"no path", readWrite(bot), bot, "files.read", `{}`, "", "path is required"},
		{"arguments that are not an object", readWrite(bot), bot, "files.read", `"x"`, "", "not a JSON object"},
		{"an unknown tool", readWrite(bot), bot, "files.delete", `{"path":"x"}`, "", "no such tool"},
		{"an act-tier tool even when granted", []org.Grant{{Capability: "calendar.write", Mode: org.ModeAllowed}}, bot, "calendar.write", `{}`, "", "not allowed"},
		{"a granted web tool", readWrite(bot), bot, "web.fetch", `{"url":"https://example.org"}`, "", ""},
		{"an ungranted web tool", readOnly, bot, "web.fetch", `{"url":"https://example.org"}`, "", "not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			d := Dispatcher{Defs: registry(t, rec), Grants: tc.grants, Root: tc.root}
			res, err := d.Execute(context.Background(), tc.tool, json.RawMessage(tc.args))
			if tc.refusal != "" {
				if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), tc.refusal) {
					t.Fatalf("err = %v, want a refusal containing %q", err, tc.refusal)
				}
				if strings.Contains(err.Error(), "\n") || !strings.HasPrefix(err.Error(), tc.tool+" ") {
					t.Fatalf("refusal %q is not one line naming the tool", err)
				}
				if rec.last != nil {
					t.Fatal("the executor ran after a refusal")
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if res.Text != "ran "+tc.tool || rec.last == nil {
				t.Fatalf("result %q, executor ran %v", res.Text, rec.last != nil)
			}
			if rec.last.Path != tc.wantPath {
				t.Fatalf("path = %q, want %q", rec.last.Path, tc.wantPath)
			}
			if tc.wantPath != "" && rec.last.Folder != inBot {
				t.Fatalf("folder = %q, want %q", rec.last.Folder, inBot)
			}
		})
	}
}

func TestDispatcher_Tools(t *testing.T) {
	folder := t.TempDir()
	rec := &recorder{}
	cases := []struct {
		name   string
		grants []org.Grant
		want   []string
	}{
		{"no grants", nil, nil},
		{"the defaults", []org.Grant{
			{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: folder},
			{Capability: org.FilesDelete, Mode: org.ModeAsk, Folder: folder},
			{Capability: org.WebFetch, Mode: org.ModeAllowed},
		}, []string{"files.read", "files.write", "web.fetch"}},
		{"ask mode only", []org.Grant{{Capability: org.WebFetch, Mode: org.ModeAsk}}, nil},
		{"a files grant with no folder", []org.Grant{{Capability: org.FilesRead, Mode: org.ModeAllowed}}, nil},
		{"an act-tier grant", []org.Grant{{Capability: "calendar.write", Mode: org.ModeAllowed}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Dispatcher{Defs: registry(t, rec), Grants: tc.grants, Root: folder}
			var got []string
			for _, def := range d.Tools() {
				got = append(got, def.Name)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Tools = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	rec := &recorder{}
	read := rec.def("files.read", org.FilesRead, TierRead)
	if _, err := NewRegistry(read, read); err == nil {
		t.Fatal("NewRegistry accepted a name twice")
	}
	if _, err := NewRegistry(Definition{Name: "x"}); err == nil {
		t.Fatal("NewRegistry accepted a definition with no executor")
	}
	var r Registry
	if err := r.Add(read); err != nil {
		t.Fatal(err)
	}
	if got, ok := r.Lookup("files.read"); !ok || got.Name != "files.read" {
		t.Fatalf("Lookup = %v, %v", got.Name, ok)
	}
	if _, ok := r.Lookup("web.fetch"); ok {
		t.Fatal("Lookup found a tool never added")
	}
	if all := r.All(); len(all) != 1 {
		t.Fatalf("All = %d definitions, want 1", len(all))
	}
	var none *Registry
	if none.All() != nil {
		t.Fatal("a nil registry is not empty")
	}
}
