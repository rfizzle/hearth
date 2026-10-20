package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

// On a file system that folds case (macOS), a reserved folder named in
// another case is the same folder and is refused all the same. Elsewhere
// that name is a different folder, so the test has nothing to show.
func TestDispatcher_ReservedFoldersAreRefusedInAnyCase(t *testing.T) {
	home := t.TempDir()
	reserved := []string{
		filepath.Join(home, "Inbox"),
		filepath.Join(home, "Templates"),
		filepath.Join(home, "Library", "Hearth"),
	}
	for _, r := range reserved {
		mkdir(t, r)
	}
	if _, err := os.Stat(filepath.Join(home, "templates")); err != nil {
		t.Skip("this file system tells Templates from templates")
	}
	d := Dispatcher{
		Defs:     registry(t, &recorder{}),
		Grants:   []org.Grant{{Capability: org.FilesWrite, Mode: org.ModeAllowed, Folder: home}},
		Root:     home,
		Reserved: reserved,
	}
	for _, path := range []string{
		filepath.Join(home, "templates", "x.md"),
		filepath.Join(home, "TEMPLATES"),
		filepath.Join(home, "inbox", "x.md"),
		filepath.Join(home, "INBOX", "x.md"),
		filepath.Join(home, "library", "hearth", "hearth.db"),
		filepath.Join(home, "Library", "HEARTH", "keys", "openai"),
		"../templates/x.md",
	} {
		if _, err := d.Check("files.write", json.RawMessage(`{"path":"`+path+`"}`)); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: err = %v, want a refusal", path, err)
		}
	}
}
