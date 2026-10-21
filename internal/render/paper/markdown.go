package paper

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/fonts"
)

// The limits on one render.
// See docs/decisions.md for D41.
const (
	// MaxSource is the largest Markdown file rendered.
	MaxSource = 1 << 20
	// Deadline is how long a render may take.
	Deadline = 60 * time.Second
	// maxDepth is how deeply quotes and lists nest before the rest of a
	// branch is left out; the walk recurses, and a crafted file could
	// otherwise nest a hundred thousand deep.
	maxDepth = 24
)

// An Opener opens the file an image's destination names, a relative path
// with slashes, or refuses it. It is the only way a render reaches a file;
// the caller decides which files it may reach.
type Opener func(dest string) (io.ReadCloser, error)

// Options are what a render is handed beside the Markdown.
type Options struct {
	Paper Paper
	// Name is the source as the owner reads it, such as report.md: the
	// notices name it, and its stem is the title when there is no # heading.
	Name string
	// Modified is the source's modification time: the PDF's dates.
	Modified time.Time
	// Image opens an image the document names; nil leaves every image out.
	Image Opener
}

// Report is what a render made: the pages, the PDF's size, and the notices
// for what was left out, fixed sentences that name nothing from the
// document but its file name.
type Report struct {
	Pages   int
	Bytes   int
	Notices []string
}

// ErrTooLarge is a source over MaxSource.
var ErrTooLarge = fmt.Errorf("a Markdown file over %d MB is not rendered", MaxSource>>20)

// Markdown renders src as a PDF in the print style. The same src and options
// give the same bytes: nothing reads the clock, and the dates are Modified.
// A render that runs past Deadline returns the context's error; a fault in
// the drawing library is ErrRender, never a crash of the caller.
// See docs/decisions.md for D39.
func Markdown(ctx context.Context, src []byte, o Options) (out []byte, rep Report, err error) {
	if len(src) > MaxSource {
		return nil, Report{}, ErrTooLarge
	}
	// The parser cannot be interrupted, and some inputs cost it the square
	// of their size; those are refused before it sees them.
	if err := deliverable.CheckMarkdown(src); err != nil {
		return nil, Report{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, Deadline)
	defer cancel()
	defer func() {
		if recover() != nil {
			out, rep, err = nil, Report{}, ErrRender
		}
	}()
	d, err := New(o.Paper, false, Print())
	if err != nil {
		return nil, Report{}, err
	}
	tree, err := parse(ctx, src)
	if err != nil {
		return nil, Report{}, err
	}
	w := &walker{d: d, src: src, o: o, ctx: ctx}
	w.blocks(tree, 0)
	if err := ctx.Err(); err != nil {
		return nil, Report{}, err
	}
	if d.Full() {
		w.notice("[%s runs past %d pages; the PDF stops at page %d.]", MaxPages, MaxPages)
	}
	if w.title == "" {
		w.title = strings.TrimSuffix(path.Base(o.Name), path.Ext(o.Name))
	}
	out, pages, err := d.Finish(w.title, o.Modified)
	if err != nil {
		return nil, Report{}, err
	}
	return out, Report{Pages: pages, Bytes: len(out), Notices: w.notices}, nil
}

// Markdown draws src, the prose of a shaped deliverable's text or callout,
// in the column between the current margins from the current height. It is
// checked and parsed as a Markdown file is, under ctx; a # reads as ##, since
// the deliverable's title is the only one, and an image is never opened.
func (d *Document) Markdown(ctx context.Context, src []byte) error {
	if len(src) > MaxSource {
		return ErrTooLarge
	}
	if err := deliverable.CheckMarkdown(src); err != nil {
		return err
	}
	tree, err := parse(ctx, src)
	if err != nil {
		return err
	}
	w := &walker{d: d, src: src, ctx: ctx, shaped: true}
	w.blocks(tree, 0)
	return ctx.Err()
}

// parse runs the parser on its own goroutine and waits for it or for ctx,
// so a render returns at its deadline even if the parse is still running;
// CheckMarkdown bounds how long that can be. A parser panic is ErrRender.
func parse(ctx context.Context, src []byte) (ast.Node, error) {
	done := make(chan ast.Node, 1)
	go func() {
		defer func() {
			if recover() != nil {
				done <- nil
			}
		}()
		done <- deliverable.ParseMarkdown(src)
	}()
	select {
	case tree := <-done:
		if tree == nil {
			return nil, ErrRender
		}
		return tree, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// walker draws one parsed document onto d.
type walker struct {
	d          *Document
	src        []byte
	o          Options
	ctx        context.Context
	title      string
	notices    []string
	images     int
	widths     map[int]bool // the widths of the images placed
	imageBytes int          // the image data placed so far
	tight      bool
	shaped     bool    // prose inside a deliverable: headings demoted, no title
	size       float64 // the size and leading inline text is set at
	lead       float64
}

// stopped reports whether drawing must end: the deadline passed or the
// pages ran out.
func (w *walker) stopped() bool { return w.ctx.Err() != nil || w.d.Full() }

// notice adds a fixed sentence once. Its one verb is the file's name.
func (w *walker) notice(format string, args ...any) {
	n := fmt.Sprintf(format, append([]any{w.o.Name}, args...)...)
	for _, have := range w.notices {
		if have == n {
			return
		}
	}
	w.notices = append(w.notices, n)
}

func (w *walker) blocks(parent ast.Node, depth int) {
	if depth > maxDepth {
		w.notice("[%s nests lists or quotes too deeply; the innermost were left out.]")
		return
	}
	for n := parent.FirstChild(); n != nil && !w.stopped(); n = n.NextSibling() {
		w.block(n, depth)
	}
}

func (w *walker) block(n ast.Node, depth int) {
	s := w.d.Style
	switch n := n.(type) {
	case *ast.Heading:
		w.heading(n)
	case *ast.Paragraph, *ast.TextBlock:
		w.paragraph(n, s.Body, s.Leading, "")
	case *ast.ThematicBreak:
		w.d.Space(s.Gap)
		w.d.Rule()
		w.d.Space(s.Gap)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		w.code(n)
	case *ast.Blockquote:
		w.quote(n, depth)
	case *ast.List:
		w.list(n, depth)
	case *ast.HTMLBlock:
		w.html()
	case *east.Table:
		w.table(n)
	default:
		w.blocks(n, depth+1)
	}
}

func (w *walker) html() {
	w.notice("[%s had HTML in it; it was left out of the PDF.]")
}

func (w *walker) heading(n *ast.Heading) {
	s := w.d.Style
	level := n.Level
	if w.shaped {
		level = deliverable.DemoteHeading(level)
	}
	size := s.Headings[min(max(level, 1), 6)-1]
	if level == 1 && w.title == "" {
		w.title = plain(n, w.src)
	}
	w.d.Space(2 * s.Gap)
	// A heading keeps a line of what follows it on its page.
	w.d.Need(size*1.25 + s.Leading)
	w.paragraph(n, size, size*1.25, "B")
}

// paragraph draws n's inline content at size on lines lead apart, then the
// gap below a block.
func (w *walker) paragraph(n ast.Node, size, lead float64, style string) {
	w.size, w.lead = size, lead
	w.d.Font(fonts.Sans, style, size)
	w.d.Ink(w.d.Style.Ink)
	w.inline(n, style)
	w.d.pdf.Ln(lead)
	if !w.tight {
		w.d.Space(w.d.Style.Gap)
	}
}

// inline draws the inline children of n in style ("B", "I", "U", "S" or
// "M" for code, combined).
func (w *walker) inline(parent ast.Node, style string) {
	for n := parent.FirstChild(); n != nil && !w.stopped(); n = n.NextSibling() {
		switch n := n.(type) {
		case *ast.Text:
			w.text(string(n.Segment.Value(w.src)), style)
			switch {
			case n.HardLineBreak():
				w.d.pdf.Ln(w.lead)
			case n.SoftLineBreak():
				w.text(" ", style)
			}
		case *ast.String:
			w.text(string(n.Value), style)
		case *ast.CodeSpan:
			w.inline(n, with(style, "M"))
		case *ast.Emphasis:
			w.inline(n, with(style, emphasis(n.Level)))
		case *east.Strikethrough:
			w.inline(n, with(style, "S"))
		case *ast.Link:
			w.link(n, string(n.Destination), style)
		case *ast.AutoLink:
			w.text(string(n.URL(w.src)), with(style, "U"))
		case *ast.Image:
			w.image(n)
		case *ast.RawHTML:
			w.html()
		case *east.TaskCheckBox:
			w.checkbox(n.IsChecked)
		default:
			w.inline(n, style)
		}
	}
}

// link draws the link's text underlined and, when it differs, the address
// after it in grey. A link is text only: no annotation makes it act.
func (w *walker) link(n *ast.Link, dest, style string) {
	w.inline(n, with(style, "U"))
	if dest != "" && dest != plain(n, w.src) {
		w.d.Ink(w.d.Style.Secondary)
		w.text(" ("+dest+")", strings.ReplaceAll(style, "U", ""))
		w.d.Ink(w.d.Style.Ink)
	}
}

// text draws s in style, a word at a time: fpdf's line breaking costs the
// square of the string it is given, so a long paragraph in one call would
// take minutes.
func (w *walker) text(s, style string) {
	family := fonts.Sans
	size := w.size
	if strings.Contains(style, "M") {
		family, size = fonts.Mono, w.size*0.9
	}
	w.d.Font(family, style, size)
	for _, word := range words(Clean(s)) {
		if w.stopped() {
			return
		}
		w.d.pdf.Write(w.lead, word)
	}
}

// words cuts s after each space and into pieces of at most 64 characters.
func words(s string) []string {
	var out []string
	for s != "" {
		i := strings.IndexByte(s, ' ') + 1
		if i == 0 {
			i = len(s)
		}
		if utf8.RuneCountInString(s[:i]) > 64 {
			i = 0
			for n := 0; n < 64; n++ {
				_, size := utf8.DecodeRuneInString(s[i:])
				i += size
			}
		}
		out = append(out, s[:i])
		s = s[i:]
	}
	return out
}

// checkbox draws a task's box, ticked or not, in the line.
func (w *walker) checkbox(checked bool) {
	p := w.d.pdf
	side := w.size * 0.7
	x, y := p.GetX(), p.GetY()+(w.lead-side)/2
	c := w.d.Style.Secondary
	p.SetDrawColor(c.R, c.G, c.B)
	p.SetLineWidth(w.d.Style.Hairline * 1.5)
	p.Rect(x, y, side, side, "D")
	if checked {
		p.Line(x+side*0.2, y+side*0.55, x+side*0.45, y+side*0.8)
		p.Line(x+side*0.45, y+side*0.8, x+side*0.85, y+side*0.2)
	}
	p.SetX(x + side + w.size*0.4)
}

func (w *walker) code(n ast.Node) {
	s, p := w.d.Style, w.d.pdf
	w.d.Font(fonts.Mono, "", s.Code)
	w.d.Ink(s.Ink)
	p.SetFillColor(s.Fill.R, s.Fill.G, s.Fill.B)
	p.SetCellMargin(s.Pad)
	defer p.SetCellMargin(0)
	width := w.d.Width()
	p.CellFormat(width, s.Pad/2, "", "", 2, "L", true, 0, "")
	lines := n.Lines()
	for i := 0; i < lines.Len() && !w.stopped(); i++ {
		seg := lines.At(i)
		line := strings.TrimRight(string(seg.Value(w.src)), "\r\n")
		line = Clean(strings.ReplaceAll(line, "\t", "    "))
		parts := p.SplitText(line, width)
		if len(parts) == 0 {
			parts = []string{""}
		}
		for _, part := range parts {
			if w.stopped() {
				return
			}
			p.CellFormat(width, s.CodeLeft, part, "", 2, "L", true, 0, "")
		}
	}
	p.CellFormat(width, s.Pad/2, "", "", 2, "L", true, 0, "")
	w.d.Space(s.Gap)
}

func (w *walker) quote(n *ast.Blockquote, depth int) {
	p, s := w.d.pdf, w.d.Style
	l, _, _, _ := p.GetMargins()
	w.d.OpenBar(l + s.Hairline)
	p.SetLeftMargin(l + s.Indent)
	p.SetX(l + s.Indent)
	w.blocks(n, depth+1)
	p.SetLeftMargin(l)
	p.SetX(l)
	w.d.CloseBar()
}

func (w *walker) list(n *ast.List, depth int) {
	p, s := w.d.pdf, w.d.Style
	l, _, _, _ := p.GetMargins()
	tight := w.tight
	w.tight = n.IsTight
	num := n.Start
	for item := n.FirstChild(); item != nil && !w.stopped(); item = item.NextSibling() {
		marker := "•"
		if n.IsOrdered() {
			marker = fmt.Sprintf("%d.", num)
			num++
		}
		w.d.Need(s.Leading)
		w.d.Font(fonts.Sans, "", s.Body)
		w.d.Ink(s.Ink)
		p.SetX(l)
		p.CellFormat(s.Indent+s.Pad/2, s.Leading, marker, "", 0, "L", false, 0, "")
		p.SetLeftMargin(l + s.Indent + s.Pad/2)
		w.blocks(item, depth+1)
		p.SetLeftMargin(l)
		p.SetX(l)
	}
	w.tight = tight
	if !w.tight {
		w.d.Space(s.Gap)
	}
}

// plain is the text of n's inline children with no styling.
func plain(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			b.Write(c.Segment.Value(src))
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.AutoLink:
			b.Write(c.URL(src))
		}
		return ast.WalkContinue, nil
	})
	return Clean(b.String())
}

// with is style with c added once, so nested emphasis keeps the style a few
// letters long however deep it goes.
func with(style, c string) string {
	if strings.Contains(style, c) {
		return style
	}
	return style + c
}

// emphasis is the style of *one* and **two** delimiters.
func emphasis(level int) string {
	if level >= 2 {
		return "B"
	}
	return "I"
}
