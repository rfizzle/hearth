package template

import (
	"embed"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/rfizzle/hearth/internal/deliverable"
)

//go:embed shipped/*.deliverable.json
var shippedFS embed.FS

const (
	// suffix ends the name of every template file.
	suffix = ".deliverable.json"
	// maxOwned is how many files of the owner's folder are read.
	maxOwned = 50
)

// Problem is a template file that could not be used, kept so the app can list
// it with its reason. Loading never stops for one.
type Problem struct {
	File   string // the file's name in the Templates folder
	ID     string // the id its name gives, when it gives one
	Reason string
}

// Loaded is every template that could be used, the shipped five first and the
// owner's after them, each group in id order, and every file that could not.
type Loaded struct {
	Templates []Template
	Problems  []Problem
}

// Shipped is the five templates embedded in the core, in id order.
func Shipped() []Template {
	return shipped().Templates
}

func shipped() Loaded {
	var l Loaded
	entries, _ := shippedFS.ReadDir("shipped") // names sorted; ids equal the stems
	for _, e := range entries {
		data, err := shippedFS.ReadFile("shipped/" + e.Name())
		if err == nil {
			var t Template
			if t, err = Parse(data); err == nil {
				t.Shipped = true
				l.Templates = append(l.Templates, t)
			}
		}
		if err != nil {
			l.Problems = append(l.Problems, Problem{File: e.Name(), Reason: err.Error()})
		}
	}
	return l
}

// Load returns the shipped set and then the owner's templates from
// <root>/Templates/*.deliverable.json. It reads that folder only: no
// recursion, no link followed, at most fifty files, each at most
// deliverable.MaxBytes. A file that fails to read or parse, whose name is not
// its id, or whose id is a shipped one becomes a Problem. A root of "" or a
// folder that is not there loads the shipped set alone.
func Load(root string) Loaded {
	l := shipped()
	if root == "" {
		return l
	}
	dir := filepath.Join(root, "Templates")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return l
	}
	if err != nil {
		l.Problems = append(l.Problems, Problem{File: "Templates", Reason: "the folder could not be read: " + err.Error()})
		return l
	}
	var owned []Template
	seen := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		if seen++; seen > maxOwned {
			l.Problems = append(l.Problems, Problem{File: name, Reason: "the folder holds more than 50 templates; this one and the rest were not read"})
			break
		}
		t, err := loadOwned(dir, e, l.Templates)
		if err != nil {
			l.Problems = append(l.Problems, Problem{File: name, ID: strings.TrimSuffix(name, suffix), Reason: err.Error()})
			continue
		}
		owned = append(owned, t)
	}
	slices.SortFunc(owned, func(a, b Template) int { return strings.Compare(a.ID, b.ID) })
	l.Templates = append(l.Templates, owned...)
	return l
}

func loadOwned(dir string, e fs.DirEntry, shipped []Template) (Template, error) {
	if !e.Type().IsRegular() {
		return Template{}, refuse("this is a link or a folder; a template is a plain file")
	}
	data, err := readOwned(filepath.Join(dir, e.Name()))
	if err != nil {
		return Template{}, err
	}
	t, err := Parse(data)
	if err != nil {
		return Template{}, err
	}
	if stem := strings.TrimSuffix(e.Name(), suffix); stem != t.ID {
		return Template{}, refuse("the file is named %s but the template id is %s; make them the same", stem, t.ID)
	}
	if slices.ContainsFunc(shipped, func(s Template) bool { return s.ID == t.ID }) {
		return Template{}, refuse("%s is a shipped template; pick another id", t.ID)
	}
	return t, nil
}

// readOwned reads a regular file of at most deliverable.MaxBytes, opening it
// without following a link and without waiting on a pipe.
func readOwned(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, refuse("the file could not be opened as a plain file")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, refuse("this is not a plain file")
	}
	data, err := io.ReadAll(io.LimitReader(f, deliverable.MaxBytes+1))
	if err != nil {
		return nil, refuse("the file could not be read")
	}
	if len(data) > deliverable.MaxBytes {
		return nil, refuse("the template is over %d KB; the limit is %d KB", deliverable.MaxBytes>>10, deliverable.MaxBytes>>10)
	}
	return data, nil
}
