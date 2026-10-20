package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// realTempDir is t.TempDir with its own symlinks resolved, because on macOS
// the temporary directory sits behind /var -> /private/var.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEnsure_CreatesTheRootAndInbox(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Hearth")
	for range 2 {
		if err := Ensure(root); err != nil {
			t.Fatalf("Ensure: %v", err)
		}
	}
	if info, err := os.Stat(filepath.Join(root, "Inbox")); err != nil || !info.IsDir() {
		t.Fatalf("Inbox: %v", err)
	}
	if err := Ensure("Hearth"); err == nil {
		t.Fatal("Ensure accepted a relative root")
	}
}

func TestBotFolder(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "Hearth")
	if got, want := BotFolder(root, "Research/Analyst"), filepath.Join(root, "Research-Analyst"); got != want {
		t.Fatalf("BotFolder = %q, want %q", got, want)
	}
}

func TestResolve(t *testing.T) {
	base := realTempDir(t)
	root := filepath.Join(base, "Hearth")
	outside := filepath.Join(base, "elsewhere")
	for _, dir := range []string{filepath.Join(root, "Writer"), outside} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Writer", "draft.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		filepath.Join(root, "Out"):                 outside,                                   // an ancestor that leads out
		filepath.Join(root, "Writer", "leak.txt"):  filepath.Join(outside, "secret.txt"),      // a leaf that leads out
		filepath.Join(root, "Writer", "later.txt"): filepath.Join(outside, "not-yet.txt"),     // a dangling leaf that leads out
		filepath.Join(root, "Drafts"):              filepath.Join(root, "Writer"),             // an ancestor that stays in
		filepath.Join(root, "Writer", "alias.md"):  filepath.Join(root, "Writer", "draft.md"), // a leaf that stays in
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name, requested, want string
	}{
		{"a relative file", "Writer/draft.md", filepath.Join(root, "Writer", "draft.md")},
		{"the root itself", ".", root},
		{"a file that does not exist yet", "Writer/new/report.md", filepath.Join(root, "Writer", "new", "report.md")},
		{"an absolute path inside", filepath.Join(root, "Writer"), filepath.Join(root, "Writer")},
		{"dot-dot that stays inside", "Writer/../Writer/draft.md", filepath.Join(root, "Writer", "draft.md")},
		{"a symlinked ancestor inside", "Drafts/draft.md", filepath.Join(root, "Writer", "draft.md")},
		{"a symlinked leaf inside", "Writer/alias.md", filepath.Join(root, "Writer", "draft.md")},
		{"dot-dot out", "../elsewhere/secret.txt", ""},
		{"dot-dot out of a subfolder", "Writer/../../elsewhere", ""},
		{"an absolute path outside", filepath.Join(outside, "secret.txt"), ""},
		{"a symlinked ancestor out", "Out/secret.txt", ""},
		{"a new file under a symlinked ancestor out", "Out/new.txt", ""},
		{"a symlinked leaf out", "Writer/leak.txt", ""},
		{"a dangling symlinked leaf out", "Writer/later.txt", ""},
		{"a sibling sharing the root's prefix", filepath.Join(base, "Hearth2", "x"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Resolve(root, c.requested)
			if c.want == "" {
				if err == nil {
					t.Fatalf("Resolve(%q) = %q, want a refusal", c.requested, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q): %v", c.requested, err)
			}
			if got != c.want {
				t.Fatalf("Resolve(%q) = %q, want %q", c.requested, got, c.want)
			}
		})
	}

	// The refusals that reach a path name it as outside, so a caller can
	// tell them from a disk error.
	if _, err := Resolve(root, "Out/secret.txt"); !errors.Is(err, ErrOutside) {
		t.Fatalf("symlinked ancestor out: %v, want ErrOutside", err)
	}
}

func TestResolve_ARootBehindASymlink(t *testing.T) {
	base := realTempDir(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "link")
	if err := os.Symlink(real, root); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve(root, "notes.md")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(real, "notes.md"); got != want {
		t.Fatalf("Resolve = %q, want %q", got, want)
	}
	if _, err := Resolve("relative", "x"); err == nil {
		t.Fatal("Resolve accepted a relative root")
	}
}

func TestProjectFolder_IsOneElementUnderProjects(t *testing.T) {
	root := realTempDir(t)
	for _, name := range []string{"Launch", "../x", "a/b", "..", "Inbox"} {
		got := ProjectFolder(root, name)
		if filepath.Dir(got) != filepath.Join(root, "Projects") {
			t.Errorf("ProjectFolder(%q) = %q, want one element under Projects", name, got)
		}
	}
}

func TestEnsureFolder(t *testing.T) {
	root := realTempDir(t)
	folder := ProjectFolder(root, "../x")
	for range 2 {
		if err := EnsureFolder(root, folder); err != nil {
			t.Fatalf("EnsureFolder: %v", err)
		}
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		t.Fatalf("folder: %v", err)
	}
	if err := EnsureFolder(root, filepath.Join(root, "..", "elsewhere")); !errors.Is(err, ErrOutside) {
		t.Fatalf("EnsureFolder outside the root = %v, want ErrOutside", err)
	}
	if err := EnsureFolder(root, "Projects/../../elsewhere"); !errors.Is(err, ErrOutside) {
		t.Fatalf("EnsureFolder escaping = %v, want ErrOutside", err)
	}
}
