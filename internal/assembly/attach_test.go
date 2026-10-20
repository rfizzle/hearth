package assembly_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rfizzle/hearth/internal/assembly"
	"github.com/rfizzle/hearth/internal/org"
)

// drop is a Hearth folder, a folder the bot may read, a folder hearth keeps
// for itself, and a folder outside all three, each holding report.txt.
type drop struct {
	root, readable, reserved, outside string
}

func newDrop(t *testing.T) drop {
	t.Helper()
	dir := t.TempDir()
	d := drop{
		root: filepath.Join(dir, "Hearth"), readable: filepath.Join(dir, "Reports"),
		reserved: filepath.Join(dir, "Support"), outside: filepath.Join(dir, "Desktop"),
	}
	for _, f := range []string{d.readable, d.reserved, d.outside} {
		if err := os.MkdirAll(f, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f, "report.txt"), []byte("in "+filepath.Base(f)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func (d drop) attach(paths ...string) ([]org.FileRef, error) {
	return assembly.Attach(d.root, []string{d.readable}, []string{d.reserved}, paths)
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAttach_NamesShowPlainOnOneLine(t *testing.T) {
	d := newDrop(t)
	odd := filepath.Join(d.readable, "re\u200bport\u202e\n(ignore).txt")
	if err := os.WriteFile(odd, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	refs, err := d.attach(odd)
	if err != nil || len(refs) != 1 {
		t.Fatalf("attach = %v, %v", refs, err)
	}
	if want := "report (ignore).txt"; refs[0].Name != want {
		t.Errorf("name = %q, want %q", refs[0].Name, want)
	}
}

func TestAttach_CopiesWhatTheBotCannotRead(t *testing.T) {
	d := newDrop(t)
	outside := filepath.Join(d.outside, "report.txt")
	refs, err := d.attach(outside, outside, filepath.Join(d.readable, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	inbox, _ := filepath.EvalSymlinks(filepath.Join(d.root, "Inbox"))
	readable, _ := filepath.EvalSymlinks(filepath.Join(d.readable, "report.txt"))
	want := []org.FileRef{
		{Path: filepath.Join(inbox, "report.txt"), Name: "report.txt"},
		{Path: filepath.Join(inbox, "report 2.txt"), Name: "report 2.txt"},
		{Path: readable, Name: "report.txt"},
	}
	if len(refs) != len(want) {
		t.Fatalf("refs = %+v", refs)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("ref %d = %+v, want %+v", i, refs[i], want[i])
		}
	}
	if read(t, want[1].Path) != "in Desktop" {
		t.Error("the copy does not hold the file")
	}
	// A file already in the Inbox stays where it is.
	again, err := d.attach(want[0].Path)
	if err != nil || again[0].Path != want[0].Path {
		t.Errorf("attach from the Inbox = %+v, %v; want it left there", again, err)
	}
}

func TestAttach_NeverWritesThroughALink(t *testing.T) {
	d := newDrop(t)
	if _, err := d.attach(); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(d.root, "Inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(d.outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(d.root, "Inbox", "report.txt")); err != nil {
		t.Fatal(err)
	}
	refs, err := d.attach(filepath.Join(d.readable, "..", "Desktop", "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(refs[0].Path) != "report 2.txt" || read(t, victim) != "keep" {
		t.Errorf("ref = %+v, victim %q; want a new name and the link's target untouched", refs[0], read(t, victim))
	}
}

func TestAttach_Refuses(t *testing.T) {
	d := newDrop(t)
	big := filepath.Join(d.outside, "disk.img")
	if err := os.WriteFile(big, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, 64<<20+1); err != nil { // sparse: no disk used
		t.Fatal(err)
	}
	link := filepath.Join(d.outside, "shortcut.txt")
	if err := os.Symlink(filepath.Join(d.reserved, "report.txt"), link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"a folder":              d.outside,
		"a missing path":        filepath.Join(d.outside, "nope.txt"),
		"a relative path":       "report.txt",
		"a reserved file":       filepath.Join(d.reserved, "report.txt"),
		"a link to a reserved":  link,
		"a reserved via dotdot": filepath.Join(d.outside, "..", "Support", "report.txt"),
		"a file over 64 MB":     big,
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			refs, err := d.attach(filepath.Join(d.outside, "report.txt"), p)
			if !errors.Is(err, assembly.ErrAttachment) || refs != nil {
				t.Fatalf("Attach = %+v, %v; want ErrAttachment and nothing", refs, err)
			}
		})
	}
	if _, err := d.attach(big); err == nil || err.Error() != "disk.img is larger than 64 MB; share a folder instead: "+assembly.ErrAttachment.Error() {
		t.Errorf("a large file is refused with %v", err)
	}
	if _, err := assembly.Attach("", nil, nil, []string{filepath.Join(d.outside, "report.txt")}); err == nil {
		t.Error("attached with no Hearth folder")
	}
}

func TestInboxGrant(t *testing.T) {
	g, ok := assembly.InboxGrant(org.Org{WorkspacePath: "/ws"}, "b1")
	if !ok || g != (org.Grant{BotID: "b1", Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: "/ws/Inbox"}) {
		t.Errorf("InboxGrant = %+v, %v", g, ok)
	}
	if _, ok := assembly.InboxGrant(org.Org{}, "b1"); ok {
		t.Error("a grant on the Inbox of an org with no folder")
	}
}
