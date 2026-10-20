package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/document"
	"github.com/rfizzle/hearth/internal/tools"
)

// Options are what the file tools are handed beyond the folder they work in.
type Options struct {
	// Recogniser reads the pictures in a document while the app is open; nil
	// is the command-line helper, and a picture page is then a notice.
	Recogniser document.Recogniser
}

// kindOf is what f, the file called name, is read as. Reading a text file
// and refusing a name that disagrees with its bytes both start here.
func kindOf(f *os.File, name string) (document.Kind, error) {
	head := make([]byte, 512)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", failed("files.read", "read", name, err)
	}
	kind, err := document.Detect(name, head[:n])
	if err != nil {
		return "", fmt.Errorf("files.read: %w", err)
	}
	return kind, nil
}

// readDocument reads the document at call.Path as text with page markers.
// Extraction stops once the window past the offset is filled, so a long
// document costs what the model reads of it; offset counts bytes of the
// extracted text, and pages picks a range to read it from.
func (o Options) readDocument(ctx context.Context, call tools.Call, a readArgs, name string) (tools.Result, error) {
	name = document.OneLine(name)
	pages, err := document.ParseRange(a.Pages)
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	doc, err := document.Read(ctx, call.Path, document.Options{
		Name: name, Pages: pages, Sheet: a.Sheet, Want: int(min(a.Offset, 1<<30)) + readCap + utf8.UTFMax, Recogniser: o.Recogniser,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("files.read: %w", err)
	}
	if doc.Text == "" { // a refusal in notices: a password, an empty file, a time limit
		return tools.Result{Text: strings.Join(doc.Notices, "\n")}, nil
	}
	text := doc.Text
	if a.Offset > int64(len(text)) {
		return tools.Result{}, fmt.Errorf("files.read: offset %d is past the end of the text of %s (%d bytes)", a.Offset, name, len(text))
	}
	more := a.Offset+readCap < int64(len(text))
	buf := []byte(text[a.Offset:])
	if more {
		buf = buf[:readCap]
	}
	skipped, window := wholeRunes(buf, more)
	from := a.Offset + int64(skipped)
	end := from + int64(len(window))

	var out strings.Builder
	if lead, ok := describe(doc); ok {
		fmt.Fprintf(&out, "[File: %s, %s.]\n\n", name, lead)
	}
	out.Write(window)
	next := ""
	if doc.Next > 0 {
		next = fmt.Sprintf(`pages "%s"`, document.PagesHint(doc.Next, doc.PageCount))
	}
	switch {
	case end < int64(len(text)) && next != "":
		fmt.Fprintf(&out, "\n\n[Showing bytes %d to %d. To read on, call files.read with offset %d, or with %s.]", from, end, end, next)
	case end < int64(len(text)):
		fmt.Fprintf(&out, "\n\n[Showing bytes %d to %d of %d. To read on, call files.read with offset %d.]", from, end, len(text), end)
	case next != "":
		fmt.Fprintf(&out, "\n\n[To read on, call files.read with %s.]", next)
	case from > 0:
		fmt.Fprintf(&out, "\n\n[Showing bytes %d to %d of %d, the end of the text.]", from, end, len(text))
	}
	for _, n := range doc.Notices {
		out.WriteString("\n\n" + n)
	}
	return tools.Result{Text: out.String()}, nil
}

// describe is what the first line says of a document: its size and where the
// text came from. A kind that says it all in its text, a picture, has none.
func describe(doc document.Document) (string, bool) {
	switch doc.Kind {
	case document.KindPPTX:
		return plural(doc.PageCount, "slide") + ", read from the slides' text", true
	case document.KindDocx:
		return "a Word document, read as text", true
	case document.KindCSV:
		return "a table, read as text", true
	case document.KindPDF:
		return plural(doc.PageCount, "page") + ", read from its text layer", true
	}
	return doc.Header, doc.Header != ""
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
