package files

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

// bot is a bot's folder with read and write granted in it, the files tools
// behind a dispatcher, and a folder outside it.
type bot struct {
	folder, outside string
	d               tools.Dispatcher
}

func newBot(t *testing.T, grants ...org.Capability) bot {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b := bot{folder: filepath.Join(base, "Writer"), outside: filepath.Join(base, "elsewhere")}
	for _, dir := range []string{b.folder, b.outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	reg, err := tools.NewRegistry(Definitions(Options{})...)
	if err != nil {
		t.Fatal(err)
	}
	var gs []org.Grant
	for _, c := range grants {
		gs = append(gs, org.Grant{Capability: c, Mode: org.ModeAllowed, Folder: b.folder})
	}
	b.d = tools.Dispatcher{Defs: reg, Grants: gs, Root: b.folder}
	return b
}

func (b bot) put(t *testing.T, name string, content []byte) {
	t.Helper()
	path := filepath.Join(b.folder, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fifo makes a named pipe in the folder, which opening would block on.
func (b bot) fifo(t *testing.T, name string) {
	t.Helper()
	if err := syscall.Mkfifo(filepath.Join(b.folder, name), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (b bot) call(name string, args any) (tools.Result, error) {
	raw, _ := json.Marshal(args)
	return b.d.Execute(context.Background(), name, raw)
}

func TestRead(t *testing.T) {
	b := newBot(t, org.FilesRead)
	long := strings.Repeat("a", readCap+100)
	// A two-byte character straddles the cap.
	split := strings.Repeat("a", readCap-1) + "é" + "tail"
	b.put(t, "brief.md", []byte("# Brief\n"))
	b.put(t, "long.md", []byte(long))
	b.put(t, "split.md", []byte(split))
	b.put(t, "empty.md", nil)
	b.put(t, "logo.dat", []byte{0x89, 'P', 'N', 'G', 0, 0, 0, 0})
	b.put(t, "sub/notes.md", []byte("notes"))
	b.fifo(t, "pipe")

	cases := []struct {
		name   string
		args   map[string]any
		want   string // the whole text, or a prefix when notice is set
		notice string // a fragment the text ends with
		err    string
	}{
		{"a short file", map[string]any{"path": "brief.md"}, "# Brief\n", "", ""},
		{"a file in a subfolder", map[string]any{"path": "sub/notes.md"}, "notes", "", ""},
		{"an empty file", map[string]any{"path": "empty.md"}, "(the file is empty)", "", ""},
		{"a long file is cut at the cap", map[string]any{"path": "long.md"}, long[:readCap], "call files.read with offset 65536.]", ""},
		{"and read on from the offset", map[string]any{"path": "long.md", "offset": readCap}, long[readCap:], "the end of the file.]", ""},
		{"a character the cap would split", map[string]any{"path": "split.md"}, split[:readCap-1], "with offset 65535.]", ""},
		{"an offset inside a character", map[string]any{"path": "split.md", "offset": readCap}, "tail", "bytes 65537 to 65541 of 65541, the end of the file.]", ""},
		{"a binary file", map[string]any{"path": "logo.dat"}, "", "", "not a text file"},
		{"a folder", map[string]any{"path": "sub"}, "", "", "is a folder"},
		{"a missing file", map[string]any{"path": "nope.md"}, "", "", "could not read nope.md: no such file"},
		{"a named pipe", map[string]any{"path": "pipe"}, "", "", "pipe is not a regular file"},
		{"an offset past the end", map[string]any{"path": "brief.md", "offset": 100}, "", "", "past the end"},
		{"a negative offset", map[string]any{"path": "brief.md", "offset": -1}, "", "", "0 or more"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := b.call("files.read", tc.args)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) || strings.Contains(err.Error(), b.folder) {
					t.Fatalf("err = %v, want %q and no absolute path", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("files.read: %v", err)
			}
			if tc.notice == "" && res.Text != tc.want {
				t.Fatalf("text = %q, want %q", res.Text, tc.want)
			}
			if tc.notice != "" && (!strings.HasPrefix(res.Text, tc.want+"\n\n[") || !strings.HasSuffix(res.Text, tc.notice)) {
				t.Fatalf("text ends %q, want %d bytes then a notice ending %q", tail(res.Text), len(tc.want), tc.notice)
			}
			if len(res.Files) != 0 {
				t.Fatalf("a read returned files %v", res.Files)
			}
		})
	}
}

func tail(s string) string {
	if len(s) > 120 {
		return s[len(s)-120:]
	}
	return s
}

func TestWrite(t *testing.T) {
	b := newBot(t, org.FilesRead, org.FilesWrite)
	if err := os.Symlink(b.outside, filepath.Join(b.folder, "out")); err != nil {
		t.Fatal(err)
	}
	b.put(t, "old.md", []byte("a much longer old text"))
	b.fifo(t, "wpipe")

	cases := []struct {
		name string
		path string
		want string // the absolute path written; "" when refused
		err  string
	}{
		{"a new file", "brief.md", filepath.Join(b.folder, "brief.md"), ""},
		{"creates the folders above it", "drafts/2026/plan.md", filepath.Join(b.folder, "drafts", "2026", "plan.md"), ""},
		{"replaces a file", "old.md", filepath.Join(b.folder, "old.md"), ""},
		{"through a symlink out", "out/leak.md", "", "outside the folders"},
		{"onto a folder", "drafts", "", "drafts is a folder"},
		{"onto a named pipe", "wpipe", "", "wpipe is not a regular file"},
		{"under a file", "old.md/x.md", "", "old.md/x.md is outside the folders"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := b.call("files.write", map[string]any{"path": tc.path, "content": "hello"})
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) || strings.Contains(err.Error(), b.folder) {
					t.Fatalf("err = %v, want %q and no absolute path", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("files.write: %v", err)
			}
			got, err := os.ReadFile(tc.want)
			if err != nil || string(got) != "hello" {
				t.Fatalf("on disk: %q, %v", got, err)
			}
			want := []org.FileRef{{Path: tc.want, Name: filepath.Base(tc.want)}}
			if len(res.Files) != 1 || res.Files[0] != want[0] {
				t.Fatalf("files = %v, want %v", res.Files, want)
			}
			if !strings.HasSuffix(res.Text, " to "+tc.path+".") || strings.Contains(res.Text, b.folder) {
				t.Fatalf("text %q does not name %s relative to the folder", res.Text, tc.path)
			}
		})
	}
	entries, err := os.ReadDir(b.outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the folder outside holds %d entries, %v", len(entries), err)
	}
}

// TestWrite_ALinkThatAppearsAfterTheCheck runs the executor with a call the
// dispatcher checked before a symlink took the path's place.
func TestWrite_ALinkThatAppearsAfterTheCheck(t *testing.T) {
	b := newBot(t)
	write := Definitions(Options{})[1].Execute
	args := json.RawMessage(`{"path":"x","content":"secret"}`)
	if err := os.Symlink(filepath.Join(b.outside, "leak.md"), filepath.Join(b.folder, "leak.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b.outside, filepath.Join(b.folder, "dir")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		call tools.Call
	}{
		{"a link at the file", tools.Call{Args: args, Path: filepath.Join(b.folder, "leak.md"), Folder: b.folder}},
		{"a link above the file", tools.Call{Args: args, Path: filepath.Join(b.folder, "dir", "leak.md"), Folder: b.folder}},
		{"no folder to check against", tools.Call{Args: args, Path: filepath.Join(b.folder, "new.md")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := write(context.Background(), tc.call); err == nil || !strings.Contains(err.Error(), "nothing was written") {
				t.Fatalf("err = %v, want a refusal", err)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(b.outside, "leak.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a file appeared outside the folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.folder, "new.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a write with no folder went ahead: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	defs := Definitions(Options{})
	read, write := defs[0].Describe, defs[1].Describe
	cases := []struct {
		got, want string
	}{
		{read(json.RawMessage(`{"path":"brief.md"}`)), "Read brief.md"},
		{read(json.RawMessage(`{"path":"long.md","offset":65536}`)), "Read long.md from byte 65536"},
		{read(json.RawMessage(`not json`)), "Read "},
		{write(json.RawMessage(`{"path":"brief.md","content":"` + strings.Repeat("x", 2048) + `"}`)), "Write brief.md (2 KB)"},
		{write(json.RawMessage(`{"path":"a.md","content":"hi"}`)), "Write a.md (2 bytes)"},
		{write(json.RawMessage(`{"path":"a.md","content":"h"}`)), "Write a.md (1 byte)"},
		{write(json.RawMessage(`{"path":"big.md","content":"` + strings.Repeat("x", 3<<20/2) + `"}`)), "Write big.md (1.5 MB)"},
		{write(json.RawMessage(`{"path":"a.md"}`)), "Write a.md"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("describe = %q, want %q", tc.got, tc.want)
		}
	}
	for _, d := range defs {
		if !strings.Contains(d.Description, "relative to your folder") {
			t.Errorf("%s: the description does not say paths are relative to the folder", d.Name)
		}
		if !json.Valid(d.Parameters) {
			t.Errorf("%s: the parameters are not valid JSON", d.Name)
		}
	}
}
