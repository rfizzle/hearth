package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The caps of one read. They are the owner's numbers: a file the attachment
// cap lets in, 100 pages, 2 MB of text, a minute.
const (
	MaxFile   = 64 << 20
	MaxPages  = 100
	MaxText   = 2 << 20
	TimeLimit = 60 * time.Second
)

// headLen is how many bytes of the file Detect looks at.
const headLen = 512

// Kind is what a file is read as.
type Kind string

// KindText is a file read as plain text, which this package does not read.
const KindText Kind = "text"

// ErrNotDocument is Read on a file that is plain text.
var ErrNotDocument = errors.New("the file is plain text, not a document")

// Range is the pages asked for, counted from 1. The zero value is all of
// them; a zero To is the last page.
type Range struct{ From, To int }

// ParseRange reads "4" or "4-12". An empty string is every page.
func ParseRange(s string) (Range, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Range{}, nil
	}
	bad := errors.New(`pages must be one page or a range such as "4-12", counted from 1`)
	from, to, isRange := strings.Cut(s, "-")
	a, err := strconv.Atoi(strings.TrimSpace(from))
	if err != nil || a < 1 {
		return Range{}, bad
	}
	if !isRange {
		return Range{From: a, To: a}, nil
	}
	b, err := strconv.Atoi(strings.TrimSpace(to))
	if err != nil || b < a {
		return Range{}, bad
	}
	return Range{From: a, To: b}, nil
}

// Options is what one read is asked for.
type Options struct {
	// Name is the file as the model knows it, relative to its folder; empty
	// is the file's base name. The absolute path never appears in a result.
	Name string
	// Pages limits a paged kind to a range.
	Pages Range
	// Sheet chooses one sheet of a workbook, by name or by its number from 1;
	// empty reads the first sheets.
	Sheet string
	// Want is how many bytes of text the caller will use. A reader stops once
	// it has that many, so a long document costs what is read of it; 0 reads
	// to the caps.
	Want int
	// Limit is the wall time of the read; 0 is TimeLimit.
	Limit time.Duration
	// Recogniser reads picture pages; nil leaves them unread. It is not
	// consulted yet.
	Recogniser Recogniser
}

// Page is one page of a paged document.
type Page struct {
	Number int
	// Text is the page's cleaned text; empty for a picture or an unread page.
	Text string
	// Picture is a page the picture rule marks, Unread one the parser could
	// not read or the read never reached before it stopped.
	Picture, Unread bool
}

// Document is what a read returns.
type Document struct {
	Kind Kind
	// PageCount is the pages the file claims; 0 for a kind without pages and
	// for a file that was refused.
	PageCount int
	Pages     []Page
	// Text is the pages' text in order, each after its "[Page n]" marker.
	Text string
	// Notices are fixed sentences in brackets about what was left out.
	Notices []string
	// Header is what the first line says after the file name for a kind the
	// caller has no words for, such as "3 sheets, read as tables"; empty when
	// the text says it all, as a picture's does.
	Header string
	// Next is the first page of the range the read stopped before because
	// Want was met; 0 when it did not.
	Next int
}

// Recognised is the text a Recogniser found on one page.
type Recognised struct {
	Page int
	Text string
	// FromTextLayer is a page the recogniser read from the text layer itself
	// rather than by looking at it.
	FromTextLayer bool
}

// Recogniser reads the pages that are pictures. The app provides one while it
// is open; a nil Recogniser is the command-line helper.
type Recogniser interface {
	Recognise(ctx context.Context, path string, pages []int) ([]Recognised, error)
}

// reader reads one kind from f, size bytes long. name is already safe to put
// in a notice.
type reader func(ctx context.Context, f io.ReaderAt, size int64, name string, o Options) (Document, error)

// Read reads the document at path as the kind its name and first bytes say.
// A refusal (a name that disagrees with the bytes, a file over MaxFile, a
// file the parser cannot open) is an error in words the model can read, and
// a password-protected file is a Document with only a notice.
func Read(ctx context.Context, path string, o Options) (Document, error) {
	name := oneLine(o.Name)
	if name == "" {
		name = oneLine(filepath.Base(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return Document{}, fmt.Errorf("%s could not be opened", name)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("%s is not a regular file", name)
	}
	if info.Size() > MaxFile {
		return Document{}, fmt.Errorf("%s is over %d MB, more than Hearth reads", name, MaxFile>>20)
	}
	head := make([]byte, headLen)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return Document{}, fmt.Errorf("%s could not be read", name)
	}
	kind, err := Detect(name, head[:n])
	if err != nil {
		return Document{}, err
	}
	row, ok := lookup(kind)
	if !ok {
		return Document{}, ErrNotDocument
	}
	limit := o.Limit
	if limit <= 0 {
		limit = TimeLimit
	}
	limited, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	o.Limit = limit
	doc, err := row.read(limited, f, info.Size(), name, o)
	if cerr := ctx.Err(); cerr != nil {
		return Document{}, cerr // the run was cancelled, not the read's own minute
	}
	return doc, err
}

// OneLine is a file name as a notice shows it, for a caller that puts the name
// in the app's voice itself.
func OneLine(s string) string { return oneLine(s) }

// oneLine is a file name as a notice shows it: no control characters, so a
// name cannot start a line of its own.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f || (r >= 0x80 && r < 0xa0) || r == 0x2028 || r == 0x2029 {
			return -1
		}
		return r
	}, s)
}
