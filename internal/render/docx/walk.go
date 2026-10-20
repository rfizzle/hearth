package docx

import (
	"context"
	"encoding/xml"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
)

// walker writes one parsed document into the body of document.xml.
type walker struct {
	src     []byte
	o       Options
	ctx     context.Context
	body    strings.Builder
	over    bool // the body reached maxBody and the rest was left out
	notices []string

	title    bool       // the first # has been written as the Title
	quoted   int        // block quotes open
	level    int        // lists open
	cont     int        // the left indent, in twentieths of a point, of a paragraph inside a list item
	item     *itemMark  // the marker the next paragraph carries
	numbered []numStart // the numbered lists, in the order they were met

	media  []media
	images int
	bytes  int // the image data kept so far
	docPr  int
}

// itemMark is the list item whose first paragraph has yet to be written.
type itemMark struct {
	numID   int
	level   int
	ordered bool
}

// numStart is a numbered list: the level it sits on and the number it starts at.
type numStart struct{ level, start int }

// look is how a run is set.
type look struct{ bold, italic, strike, code bool }

// stopped reports whether writing must end: the deadline passed or the body
// is full.
func (w *walker) stopped() bool { return w.ctx.Err() != nil || w.over }

// notice adds a fixed sentence once. Its one verb is the file's name.
func (w *walker) notice(format string, args ...any) {
	n := fmt.Sprintf(format, append([]any{w.o.Name}, args...)...)
	if !slices.Contains(w.notices, n) {
		w.notices = append(w.notices, n)
	}
}

// put appends markup to the body, or marks it full.
func (w *walker) put(s string) {
	if w.body.Len()+len(s) > maxBody {
		w.over = true
		return
	}
	w.body.WriteString(s)
}

func (w *walker) blocks(parent ast.Node, depth int) {
	if depth > maxDepth {
		w.notice("[%s nests lists or quotes too deeply; the innermost were left out.]")
		return
	}
	w.each(parent, func(n ast.Node) { w.block(n, depth) })
}

// each calls do on the children of parent until writing must stop.
func (w *walker) each(parent ast.Node, do func(ast.Node)) {
	for _, n := range children(parent) {
		if w.stopped() {
			return
		}
		do(n)
	}
}

func (w *walker) block(n ast.Node, depth int) {
	switch n := n.(type) {
	case *ast.Heading:
		w.heading(n)
	case *ast.Paragraph, *ast.TextBlock:
		w.paragraph(n)
	case *ast.ThematicBreak:
		w.flushItem()
		w.put(`<w:p><w:pPr><w:pBdr><w:bottom w:val="single" w:sz="6" w:space="1" w:color="BFBFBF"/></w:pBdr></w:pPr></w:p>`)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		w.code(n)
	case *ast.Blockquote:
		w.flushItem()
		w.quoted++
		w.blocks(n, depth+1)
		w.quoted--
	case *east.Table:
		w.table(n)
	case *ast.HTMLBlock:
		w.html()
	case *ast.List:
		w.list(n, depth)
	default:
		w.blocks(n, depth+1)
	}
}

func (w *walker) html() {
	w.notice("[%s had HTML in it; it was left out of the Word file.]")
}

func (w *walker) heading(n *ast.Heading) {
	w.flushItem()
	level := min(max(n.Level, 1), 6)
	style := fmt.Sprintf("Heading%d", level)
	if level == 1 && !w.title {
		w.title, style = true, "Title"
	}
	w.open(style, "")
	w.inline(n, look{})
	w.put(`</w:p>`)
}

// paragraph writes n's inline content as a paragraph: a list item's first
// one carries the item's marker.
func (w *walker) paragraph(n ast.Node) {
	style := ""
	if w.quoted > 0 {
		w.flushItem()
		style = "Quote"
	}
	w.open(style, "")
	w.inline(n, look{})
	w.put(`</w:p>`)
}

// flushItem writes the marker of a list item that has no paragraph to carry
// it, as an empty paragraph, so the item is not lost.
func (w *walker) flushItem() {
	if w.item == nil {
		return
	}
	w.open("", "")
	w.put(`</w:p>`)
}

// open starts a paragraph in style, aligned by jc ("" for the start). A
// paragraph that is a list item's first takes the item's style and marker;
// one inside a list or a quote is indented to match.
func (w *walker) open(style, jc string) {
	var p strings.Builder
	item := w.item
	w.item = nil
	left, hanging := w.cont+720*w.quoted, 0
	if item != nil {
		style = "ListBullet"
		if item.ordered {
			style = "ListNumber"
		}
		left, hanging = 720*(item.level+1)+720*w.quoted, 360
	}
	if style != "" {
		p.WriteString(`<w:pStyle w:val="` + style + `"/>`)
	}
	if item != nil {
		fmt.Fprintf(&p, `<w:numPr><w:ilvl w:val="%d"/><w:numId w:val="%d"/></w:numPr>`, item.level, item.numID)
	}
	if left > 0 {
		if hanging > 0 {
			fmt.Fprintf(&p, `<w:ind w:left="%d" w:hanging="%d"/>`, left, hanging)
		} else {
			fmt.Fprintf(&p, `<w:ind w:left="%d"/>`, left)
		}
	}
	if jc != "" {
		p.WriteString(`<w:jc w:val="` + jc + `"/>`)
	}
	w.put(`<w:p>`)
	if p.Len() > 0 {
		w.put(`<w:pPr>` + p.String() + `</w:pPr>`)
	}
}

// inline writes the inline children of parent as runs in look l.
func (w *walker) inline(parent ast.Node, l look) {
	w.each(parent, func(n ast.Node) { w.span(n, l) })
}

// span writes one inline node.
func (w *walker) span(n ast.Node, l look) {
	switch n := n.(type) {
	case *ast.Link:
		w.link(n, string(n.Destination), l)
	case *ast.AutoLink:
		w.run(string(n.URL(w.src)), l)
	case *ast.Image:
		w.image(n, l)
	case *ast.RawHTML:
		w.html()
	case *ast.String:
		w.run(string(n.Value), l)
	case *ast.Text:
		w.text(n, l)
	case *ast.CodeSpan:
		w.inline(n, look{l.bold, l.italic, l.strike, true})
	case *ast.Emphasis:
		w.inline(n, look{l.bold || n.Level >= 2, l.italic || n.Level < 2, l.strike, l.code})
	case *east.Strikethrough:
		w.inline(n, look{l.bold, l.italic, true, l.code})
	case *east.TaskCheckBox:
		box := "☐ "
		if n.IsChecked {
			box = "☑ "
		}
		w.run(box, l)
	default:
		w.inline(n, l)
	}
}

// text writes a run of text and the line break after it.
func (w *walker) text(n *ast.Text, l look) {
	w.run(string(n.Segment.Value(w.src)), l)
	if n.HardLineBreak() {
		w.put(`<w:r><w:br/></w:r>`)
	} else if n.SoftLineBreak() {
		w.run(" ", l)
	}
}

// children are the direct children of n.
func children(n ast.Node) []ast.Node {
	var out []ast.Node
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		out = append(out, c)
	}
	return out
}

// link writes the link's text and, when it differs, the address after it in
// brackets. A link is text only: no relationship is made, so nothing in the
// file acts when it is opened.
func (w *walker) link(n *ast.Link, dest string, l look) {
	w.inline(n, l)
	if dest != "" && clean(dest) != plain(n, w.src) {
		w.run(" ["+dest+"]", l)
	}
}

// run writes text as one run set in l.
func (w *walker) run(text string, l look) {
	text = clean(text)
	if text == "" {
		return
	}
	var rpr strings.Builder
	if l.code {
		rpr.WriteString(`<w:rStyle w:val="CodeChar"/>`)
	}
	if l.bold {
		rpr.WriteString(`<w:b/>`)
	}
	if l.italic {
		rpr.WriteString(`<w:i/>`)
	}
	if l.strike {
		rpr.WriteString(`<w:strike/>`)
	}
	w.put(`<w:r>`)
	if rpr.Len() > 0 {
		w.put(`<w:rPr>` + rpr.String() + `</w:rPr>`)
	}
	w.put(`<w:t xml:space="preserve">` + escape(text) + `</w:t></w:r>`)
}

// code writes each line of a code block as a paragraph of the Code style.
func (w *walker) code(n ast.Node) {
	w.flushItem()
	for _, line := range w.lines(n) {
		if w.stopped() {
			break
		}
		w.open("Code", "")
		w.run(line, look{})
		w.put(`</w:p>`)
	}
	w.put(`<w:p><w:pPr><w:spacing w:after="0"/></w:pPr></w:p>`)
}

// lines are the lines of a code block without their line ends, a tab as
// four spaces.
func (w *walker) lines(n ast.Node) []string {
	segs := n.Lines()
	out := make([]string, 0, segs.Len())
	for i := range segs.Len() {
		seg := segs.At(i)
		line := strings.TrimRight(string(seg.Value(w.src)), "\r\n")
		out = append(out, strings.ReplaceAll(line, "\t", "    "))
	}
	return out
}

// list writes a list's items, each on the list's level, and a numbered list
// with a numbering of its own that starts where the Markdown says.
func (w *walker) list(n *ast.List, depth int) {
	w.flushItem()
	level := min(w.level, maxLevels-1)
	numID := 1
	ordered := n.IsOrdered()
	if ordered {
		if len(w.numbered) >= maxNumbered {
			ordered = false
			w.notice("[%s has more than 1,000 numbered lists; the rest are bulleted in the Word file.]")
		} else {
			w.numbered = append(w.numbered, numStart{level, max(n.Start, 0)})
			numID = len(w.numbered) + 1
		}
	}
	w.level++
	cont := w.cont
	w.cont = 720 * (level + 1)
	for item := n.FirstChild(); item != nil && !w.stopped(); item = item.NextSibling() {
		w.item = &itemMark{numID: numID, level: level, ordered: ordered}
		w.blocks(item, depth+1)
		w.flushItem()
	}
	w.cont = cont
	w.level--
}

// table writes a table as a TableGrid table of equal columns, the header row
// in bold and repeated on each page, each cell's inline content in a
// paragraph of its own.
func (w *walker) table(t *east.Table) {
	w.flushItem()
	cols := len(t.Alignments)
	if cols == 0 {
		return
	}
	width := 9360
	if w.o.A4 {
		width = 9026
	}
	w.put(`<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/>` +
		`<w:tblLook w:val="0420" w:firstRow="1" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="0" w:noVBand="1"/></w:tblPr><w:tblGrid>`)
	for range cols {
		w.put(fmt.Sprintf(`<w:gridCol w:w="%d"/>`, width/cols))
	}
	w.put(`</w:tblGrid>`)
	for row := t.FirstChild(); row != nil && !w.stopped(); row = row.NextSibling() {
		_, header := row.(*east.TableHeader)
		w.put(`<w:tr>`)
		if header {
			w.put(`<w:trPr><w:cantSplit/><w:tblHeader/></w:trPr>`)
		}
		cell := row.FirstChild()
		for i := range cols {
			w.put(fmt.Sprintf(`<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr><w:p><w:pPr><w:spacing w:before="40" w:after="40"/><w:jc w:val="%s"/></w:pPr>`,
				width/cols, justify(t.Alignments[i])))
			if cell != nil {
				w.inline(cell, look{bold: header})
				cell = cell.NextSibling()
			}
			w.put(`</w:p></w:tc>`)
		}
		w.put(`</w:tr>`)
	}
	w.put(`</w:tbl><w:p/>`)
}

func justify(a east.Alignment) string {
	switch a {
	case east.AlignRight:
		return "right"
	case east.AlignCenter:
		return "center"
	}
	return "left"
}

// plain is the text of n's inline content with no styling, cleaned.
func plain(n ast.Node, src []byte) string {
	return clean(flat(n, src))
}

// flat is the text under n, a line break as a space.
func flat(n ast.Node, src []byte) string {
	switch n := n.(type) {
	case *ast.String:
		return string(n.Value)
	case *ast.AutoLink:
		return string(n.URL(src))
	case *ast.Text:
		text := string(n.Segment.Value(src))
		if n.SoftLineBreak() || n.HardLineBreak() {
			text += " "
		}
		return text
	}
	var parts []string
	for _, c := range children(n) {
		parts = append(parts, flat(c, src))
	}
	return strings.Join(parts, "")
}

// clean is s as a run can carry it: a tab is a space, control and format
// characters (which can reorder or hide text), the characters XML forbids and
// line breaks are dropped or made spaces.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t', r == ' ', r == ' ':
			return ' '
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), r == '￾', r == '￿':
			return -1
		}
		return r
	}, s)
}

// escape is s as XML character data.
func escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
