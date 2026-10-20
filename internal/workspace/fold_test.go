package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContainsText_FoldsCaseOnlyWhenTold(t *testing.T) {
	for _, c := range []struct {
		name       string
		root, path string
		fold, want bool
	}{
		{"same case", "/h/Templates", "/h/Templates/x.png", false, true},
		{"the folder itself", "/h/Templates", "/h/Templates", false, true},
		{"other case, case-sensitive", "/h/Templates", "/h/templates/x.png", false, false},
		{"other case, folded", "/h/Templates", "/h/templates/x.png", true, true},
		{"the folder in other case, folded", "/h/Inbox", "/h/INBOX", true, true},
		{"a sibling with the same prefix", "/h/Templates", "/h/Templates2/x", true, false},
		{"a parent", "/h/Templates", "/h", true, false},
		{"dot-dot is cleaned", "/h/Templates", "/h/x/../templates/y", true, true},
	} {
		if got := containsText(c.root, c.path, c.fold); got != c.want {
			t.Errorf("%s: containsText(%q, %q, %v) = %v, want %v", c.name, c.root, c.path, c.fold, got, c.want)
		}
	}
}

func TestInside_FollowsTheFileSystem(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "Templates")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(filepath.Join(dir, "templates"))
	folds := statErr == nil
	if got := foldsCase(root); got != folds {
		t.Errorf("foldsCase = %v, the file system folds = %v", got, folds)
	}
	if !Inside(root, filepath.Join(root, "x.png")) {
		t.Error("a file in the folder is not inside it")
	}
	if got := Inside(root, filepath.Join(dir, "templates", "x.png")); got != folds {
		t.Errorf("Inside in another case = %v, want %v", got, folds)
	}
	if Inside(filepath.Join(dir, "Missing"), filepath.Join(dir, "Missing", "x")) {
		t.Error("a folder that does not exist holds nothing")
	}
	if Inside("relative", "/x") || Inside(root, "relative") {
		t.Error("a relative path is inside nothing")
	}
}

func TestFlipCase(t *testing.T) {
	if got := flipCase("/a/Templates"); got != "/a/TemplateS" {
		t.Errorf("flipCase = %q", got)
	}
	if got := flipCase("/123"); got != "/123" {
		t.Errorf("flipCase of no letters = %q", got)
	}
}
