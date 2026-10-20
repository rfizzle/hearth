package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark/ast"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// The limits on one write.
// See docs/decisions.md for D41.
const (
	// MaxSource is the largest Markdown file written.
	MaxSource = 1 << 20
	// Deadline is how long a write may take.
	Deadline = 60 * time.Second
	// maxBody is the most document.xml a write makes; a table of a hundred
	// thousand cells is far inside it.
	maxBody = 32 << 20
	// maxDepth is how deeply quotes and lists nest before the rest of a
	// branch is left out; the walk recurses.
	maxDepth = 24
	// maxNumbered is how many numbered lists get a numbering of their own.
	maxNumbered = 1000
)

// ErrTooLarge is a source over MaxSource.
var ErrTooLarge = fmt.Errorf("a Markdown file over %d MB is not rendered", MaxSource>>20)

// ErrRender is a document that could not be written. It says nothing of the
// document's text.
var ErrRender = errors.New("the Word file could not be made from this document")

// An Opener opens the file an image's destination names, a relative path
// with slashes, or refuses it. It is the only way a write reaches a file; the
// caller decides which files it may reach.
type Opener = func(dest string) (io.ReadCloser, error)

// Options are what a write is handed beside the Markdown.
type Options struct {
	// A4 sets the page on A4 rather than US Letter.
	A4 bool
	// Name is the source as the owner reads it, such as report.md: the
	// notices name it.
	Name string
	// Image opens an image the document names; nil leaves every image out.
	Image Opener
}

// Report is what a write made: the file's size, and the notices for what was
// left out, fixed sentences that name nothing from the document but its file
// name.
type Report struct {
	Bytes   int
	Notices []string
}

// Write writes src as a Word document. The same src and options give the
// same bytes: nothing reads the clock and the parts are written in order. A
// write that runs past Deadline returns the context's error; a fault in the
// parser is ErrRender, never a crash of the caller.
// See docs/decisions.md for D37.
func Write(ctx context.Context, src []byte, o Options) (out []byte, rep Report, err error) {
	ctx, cancel := context.WithTimeout(ctx, Deadline)
	defer cancel()
	defer func() {
		if recover() != nil {
			out, rep, err = nil, Report{}, ErrRender
		}
	}()
	tree, err := parse(ctx, src)
	if err != nil {
		return nil, Report{}, err
	}
	w := &walker{src: src, o: o, ctx: ctx}
	w.blocks(tree, 0)
	if err := ctx.Err(); err != nil {
		return nil, Report{}, err
	}
	if w.over {
		w.notice("[%s is longer than a Word file made here can be; it stops partway.]")
	}
	if out, err = w.pack(); err != nil {
		return nil, Report{}, err
	}
	return out, Report{Bytes: len(out), Notices: w.notices}, nil
}

// parse checks src, then parses it on a goroutine of its own and waits for
// it or for ctx, so a write returns at its deadline even if the parse is
// still running. The parser cannot be interrupted, and some inputs cost it
// the square of their size; CheckMarkdown refuses those first and bounds the
// rest. A parser panic is ErrRender.
func parse(ctx context.Context, src []byte) (ast.Node, error) {
	if len(src) > MaxSource {
		return nil, ErrTooLarge
	}
	if err := deliverable.CheckMarkdown(src); err != nil {
		return nil, err
	}
	type parsed struct {
		tree ast.Node
		ok   bool
	}
	ch := make(chan parsed, 1)
	go func() {
		var p parsed
		defer func() {
			_ = recover() // a panic leaves p.ok false
			ch <- p
		}()
		p.tree, p.ok = deliverable.ParseMarkdown(src), true
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p := <-ch:
		if !p.ok || p.tree == nil {
			return nil, ErrRender
		}
		return p.tree, nil
	}
}

// part is one file of the package.
type part struct {
	name string
	data []byte
}

// zipTime is the time every entry carries: the first moment a zip can say.
func zipTime() time.Time { return time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC) }

// pack zips the parts, sorted by name, each with the same fixed time.
func (w *walker) pack() ([]byte, error) {
	parts := []part{
		{"[Content_Types].xml", []byte(contentTypes)},
		{"_rels/.rels", []byte(packageRels)},
		{"word/_rels/document.xml.rels", []byte(w.relationships())},
		{"word/document.xml", []byte(w.document())},
		{"word/numbering.xml", []byte(w.numbering())},
		{"word/styles.xml", []byte(buildStyles())},
	}
	for _, m := range w.media {
		parts = append(parts, part{"word/media/" + m.name, m.data})
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].name < parts[j].name })
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, p := range parts {
		method := zip.Deflate
		if strings.HasSuffix(p.name, ".jpg") || strings.HasSuffix(p.name, ".png") {
			method = zip.Store // already compressed
		}
		f, err := zw.CreateHeader(&zip.FileHeader{Name: p.name, Method: method, Modified: zipTime()})
		if err != nil {
			return nil, ErrRender
		}
		if _, err := f.Write(p.data); err != nil {
			return nil, ErrRender
		}
	}
	if err := zw.Close(); err != nil {
		return nil, ErrRender
	}
	return buf.Bytes(), nil
}

// relationships is word/_rels/document.xml.rels: the styles, the numbering
// and each image. There is no relationship of any other kind, so a link in
// the text cannot become one.
func (w *walker) relationships() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<Relationships xmlns="` + relNS + `">`)
	b.WriteString(`<Relationship Id="rId1" Type="` + nsR + `/styles" Target="styles.xml"/>`)
	b.WriteString(`<Relationship Id="rId2" Type="` + nsR + `/numbering" Target="numbering.xml"/>`)
	for i, m := range w.media {
		fmt.Fprintf(&b, `<Relationship Id="rId%d" Type="%s/image" Target="media/%s"/>`, firstImage+i, nsR, m.name)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// document is word/document.xml: the body and the page.
func (w *walker) document() string {
	pgW, pgH := 12240, 15840 // US Letter in twentieths of a point
	if w.o.A4 {
		pgW, pgH = 11906, 16838
	}
	return xmlHeader + `<w:document xmlns:w="` + nsW + `" xmlns:r="` + nsR + `" xmlns:wp="` + nsWP + `" xmlns:a="` + nsA + `" xmlns:pic="` + nsPic + `"><w:body>` +
		w.body.String() +
		fmt.Sprintf(`<w:sectPr><w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`, pgW, pgH) +
		`</w:body></w:document>`
}

// numbering is word/numbering.xml: list 1 is bullets, and each numbered list
// has a list of its own that starts where the Markdown says.
func (w *walker) numbering() string {
	var b strings.Builder
	b.WriteString(buildNumberingHead())
	b.WriteString(`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>`)
	for i, n := range w.numbered {
		fmt.Fprintf(&b, `<w:num w:numId="%d"><w:abstractNumId w:val="1"/><w:lvlOverride w:ilvl="%d"><w:startOverride w:val="%d"/></w:lvlOverride></w:num>`,
			i+2, n.level, n.start)
	}
	b.WriteString(`</w:numbering>`)
	return b.String()
}
