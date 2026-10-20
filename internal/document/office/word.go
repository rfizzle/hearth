package office

import (
	"context"
	"encoding/xml"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// WordText is what a Word file says.
type WordText struct {
	// Text is the document as Markdown: headings, lists, tables, bold and
	// italic.
	Text string
	// Images and Footnotes say the file had some, which are left out.
	Images, Footnotes bool
	// Cut is a document that went on past maxText bytes.
	Cut bool
	// CellsCut says a table went on past MaxCells cells and was cut there.
	CellsCut bool
}

// numbering is which lists are ordered, from word/numbering.xml.
type numbering struct {
	abstract map[string]string            // numId to abstractNumId
	formats  map[string]map[string]string // abstractNumId to ilvl to numFmt
}

func (n numbering) ordered(numID, lvl string) bool {
	f := n.formats[n.abstract[numID]][lvl]
	return f != "" && f != "bullet" && f != "none"
}

// readNumbering reads word/numbering.xml; a file without one has only
// bullets. Only a limit, a panic or the context ending is an error.
func (a *Archive) readNumbering(ctx context.Context) (numbering, error) {
	n := numbering{abstract: map[string]string{}, formats: map[string]map[string]string{}}
	if !a.has("word/numbering.xml") {
		return n, nil
	}
	var abs, lvl, num string
	err := a.walk(ctx, "word/numbering.xml", func(t xml.Token) error {
		se, ok := t.(xml.StartElement)
		if !ok {
			return nil
		}
		switch se.Name.Local {
		case "abstractNum":
			abs = attr(se, "abstractNumId")
			n.formats[abs] = map[string]string{}
		case "lvl":
			lvl = attr(se, "ilvl")
		case "numFmt":
			if m := n.formats[abs]; m != nil {
				m[lvl] = attr(se, "val")
			}
		case "num":
			num = attr(se, "numId")
		case "abstractNumId":
			n.abstract[num] = attr(se, "val")
		}
		return nil
	})
	if err != nil && !errors.Is(err, ErrDamaged) {
		return n, err
	}
	return n, nil
}

// span is a run of text with one look.
type span struct {
	text string
	bold bool
	ital bool
}

// para is a paragraph being read.
type para struct {
	spans []span
	style string
	numID string
	ilvl  string
	// The run being read.
	inRun, inRPr, inT bool
	bold, ital        bool
	text              strings.Builder
}

// wordWalker holds the state of one walk of word/document.xml.
type wordWalker struct {
	num     numbering
	maxText int
	paras   []*para // paragraphs open, innermost last (a text box nests one)
	skip    int     // depth inside an element whose content is not read
	tbl     int     // table depth
	ctx     context.Context
	rows    [][]string
	width   int      // the widest row of the table so far
	cells   []string // the row being read
	cell    []string // the paragraphs of the cell being read
	inCell  bool
	blocks  []string
	lists   []bool // whether blocks[i] is a list item
	size    int
	out     WordText
}

// Word walks word/document.xml into Markdown, stopping once maxText bytes
// are written. The numbering part says which lists are numbered.
func (a *Archive) Word(ctx context.Context, maxText int) (WordText, error) {
	num, err := a.readNumbering(ctx)
	if err != nil {
		return WordText{}, err
	}
	w := &wordWalker{ctx: ctx, num: num, maxText: maxText}
	err = a.walk(ctx, "word/document.xml", w.token)
	if err != nil {
		return WordText{}, err
	}
	var b strings.Builder
	for i, blk := range w.blocks {
		if i > 0 {
			if w.lists[i] && w.lists[i-1] {
				b.WriteString("\n")
			} else {
				b.WriteString("\n\n")
			}
		}
		b.WriteString(blk)
	}
	w.out.Text = b.String()
	if len(w.out.Text) > maxText {
		w.out.Text = Cut(w.out.Text, maxText)
		w.out.Cut = true
	}
	return w.out, nil
}

// Cut is s cut to at most n bytes, at a character boundary.
func Cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func (w *wordWalker) cur() *para {
	if len(w.paras) == 0 {
		return nil
	}
	return w.paras[len(w.paras)-1]
}

func (w *wordWalker) token(t xml.Token) error {
	switch t := t.(type) {
	case xml.StartElement:
		return w.start(t)
	case xml.EndElement:
		return w.end(t)
	case xml.CharData:
		if p := w.cur(); w.skip == 0 && p != nil && p.inT {
			p.text.Write(t)
		}
	}
	return nil
}

// on reports whether a val attribute switches a property on: <w:b/> and
// <w:b w:val="1"/> do, "0", "false" and "off" do not.
func on(se xml.StartElement) bool {
	switch strings.ToLower(attr(se, "val")) {
	case "0", "false", "off", "none":
		return false
	}
	return true
}

func (w *wordWalker) start(se xml.StartElement) error {
	if w.skip > 0 {
		w.skip++
		return nil
	}
	p := w.cur()
	switch se.Name.Local {
	case "Fallback": // the second copy of what an AlternateContent shows
		w.skip = 1
	case "p":
		w.paras = append(w.paras, &para{})
	case "pStyle":
		if p != nil {
			p.style = attr(se, "val")
		}
	case "numId":
		if p != nil && !p.inRun {
			p.numID = attr(se, "val")
		}
	case "ilvl":
		if p != nil && !p.inRun {
			p.ilvl = attr(se, "val")
		}
	case "r":
		if p != nil {
			p.inRun, p.bold, p.ital = true, false, false
			p.text.Reset()
		}
	case "rPr":
		if p != nil && p.inRun {
			p.inRPr = true
		}
	case "b":
		if p != nil && p.inRPr {
			p.bold = on(se)
		}
	case "i":
		if p != nil && p.inRPr {
			p.ital = on(se)
		}
	case "t":
		if p != nil && p.inRun {
			p.inT = true
		}
	case "tab":
		if p != nil && p.inRun && !p.inRPr {
			p.text.WriteString("\t")
		}
	case "br", "cr":
		if p != nil && p.inRun {
			p.text.WriteString("\n")
		}
	case "drawing", "pict", "object":
		w.out.Images = true
	case "footnoteReference", "endnoteReference":
		w.out.Footnotes = true
	case "tbl":
		w.tbl++
		if w.tbl == 1 {
			w.rows, w.width = nil, 0
		}
	case "tr":
		if w.tbl == 1 {
			w.cells = nil
		}
	case "tc":
		if w.tbl == 1 {
			w.cell, w.inCell = nil, true
		}
	}
	return nil
}

func (w *wordWalker) end(ee xml.EndElement) error {
	if w.skip > 0 {
		w.skip--
		return nil
	}
	p := w.cur()
	switch ee.Name.Local {
	case "t":
		if p != nil {
			p.inT = false
		}
	case "rPr":
		if p != nil {
			p.inRPr = false
		}
	case "r":
		if p != nil && p.inRun {
			p.spans = append(p.spans, span{p.text.String(), p.bold, p.ital})
			p.inRun, p.inT, p.inRPr = false, false, false
			p.text.Reset()
		}
	case "p":
		if p == nil {
			return nil
		}
		w.paras = w.paras[:len(w.paras)-1]
		return w.paragraph(p)
	case "tc":
		if w.tbl == 1 {
			w.inCell = false
			if w.fits(len(w.rows)+1, len(w.cells)+1) {
				w.cells = append(w.cells, strings.Join(w.cell, " "))
			}
		}
	case "tr":
		if w.tbl == 1 && w.fits(len(w.rows)+1, len(w.cells)) {
			w.width = max(w.width, len(w.cells))
			w.rows = append(w.rows, w.cells)
		}
	case "tbl":
		w.tbl--
		if w.tbl == 0 {
			rows := w.rows
			w.rows = nil
			return w.add(Table(rows), false)
		}
	}
	return nil
}

// fits reports whether a table of rows rows, padded to the widest of them
// and a row cols wide, stays within MaxCells; when it does not, the table is
// cut there and the rest of it is not kept. Table pads every row, so the
// count is rows times width, as sheet.go counts it.
func (w *wordWalker) fits(rows, cols int) bool {
	if rows*max(w.width, cols) > MaxCells {
		w.out.CellsCut = true
		return false
	}
	return true
}

// paragraph writes a finished paragraph, as a heading, a list item, a table
// cell's line or plain text.
func (w *wordWalker) paragraph(p *para) error {
	merged, err := render(w.ctx, p.spans)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(merged)
	if text == "" {
		return nil
	}
	if w.inCell {
		w.cell = append(w.cell, strings.Join(strings.Fields(text), " "))
		return nil
	}
	if level := headingLevel(p.style); level > 0 {
		return w.add(strings.Repeat("#", level)+" "+strings.Join(strings.Fields(text), " "), false)
	}
	if p.numID != "" && p.numID != "0" {
		depth, _ := strconv.Atoi(p.ilvl)
		marker := "- "
		if w.num.ordered(p.numID, p.ilvl) {
			marker = "1. "
		}
		return w.add(strings.Repeat("  ", min(max(depth, 0), 8))+marker+strings.Join(strings.Fields(text), " "), true)
	}
	return w.add(strings.ReplaceAll(text, "\n", "  \n"), false)
}

// add appends a block; it returns errStop once more than maxText bytes are
// written, which ends the walk.
func (w *wordWalker) add(block string, list bool) error {
	if block == "" {
		return nil
	}
	w.blocks = append(w.blocks, block)
	w.lists = append(w.lists, list)
	w.size += len(block) + 2
	if w.size > w.maxText {
		return errStop
	}
	return nil
}

// headingLevel is 1 to 6 for the styles Heading1 to Heading6 ("heading 2"
// too), 1 for Title, else 0.
func headingLevel(style string) int {
	s := strings.ToLower(strings.ReplaceAll(style, " ", ""))
	if s == "title" {
		return 1
	}
	if d, ok := strings.CutPrefix(s, "heading"); ok && len(d) == 1 && d[0] >= '1' && d[0] <= '6' {
		return int(d[0] - '0')
	}
	return 0
}

// render is the spans as Markdown: bold as ** and italic as _, merged across
// neighbours with the same look, with the spaces left outside the marks. The
// merge is linear, and it looks at the deadline as it goes.
func render(ctx context.Context, spans []span) (string, error) {
	type look struct {
		bold, ital bool
		text       strings.Builder
	}
	var merged []*look
	for i, s := range spans {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		if s.text == "" {
			continue
		}
		if n := len(merged); n > 0 && merged[n-1].bold == s.bold && merged[n-1].ital == s.ital {
			merged[n-1].text.WriteString(s.text)
			continue
		}
		m := &look{bold: s.bold, ital: s.ital}
		m.text.WriteString(s.text)
		merged = append(merged, m)
	}
	var b strings.Builder
	for i, m := range merged {
		if i%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return "", err
			}
		}
		text := m.text.String()
		core := strings.TrimSpace(text)
		if core == "" || (!m.bold && !m.ital) || strings.ContainsAny(core, "\n\t") {
			b.WriteString(text)
			continue
		}
		lead := text[:len(text)-len(strings.TrimLeft(text, " "))]
		trail := text[len(strings.TrimRight(text, " ")):]
		mark := ""
		if m.bold {
			mark += "**"
		}
		if m.ital {
			mark += "_"
		}
		b.WriteString(lead + mark + core + reverse(mark) + trail)
	}
	return b.String(), nil
}

func reverse(mark string) string {
	switch mark {
	case "**_":
		return "_**"
	}
	return mark
}
