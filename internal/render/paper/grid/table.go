package grid

import (
	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/fonts"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// sheet is one table being drawn: its columns at equal widths and its
// header, drawn again at the top of each page the table continues on.
type sheet struct {
	l      *layout
	d      *paper.Document
	cols   []deliverable.Column
	width  float64
	header []string
}

// table draws a table's header, rows and footer, a hairline under each row
// and the footer in bold. A number is written in its column's format.
func table(l *layout, d *paper.Document, w deliverable.Widget) {
	if len(w.Columns) == 0 {
		return
	}
	t := sheet{l: l, d: d, cols: w.Columns, width: d.Width() / float64(len(w.Columns))}
	for _, c := range w.Columns {
		t.header = append(t.header, c.Header)
	}
	// The header keeps its first row with it.
	first := t.height(t.header, "B")
	if len(w.Rows) > 0 {
		first += t.height(t.text(w.Rows[0]), "")
	}
	d.Need(first)
	t.row(t.header, "B", false)
	for _, r := range w.Rows {
		if l.stopped() {
			return
		}
		t.row(t.text(r), "", true)
	}
	if len(w.Footer) > 0 {
		t.row(t.text(w.Footer), "B", true)
	}
}

// text is a row's cells as written: a string as it is, a number in its
// column's format.
func (t sheet) text(r []any) []string {
	out := make([]string, len(t.cols))
	for i, v := range r {
		if i >= len(out) {
			break
		}
		switch v := v.(type) {
		case string:
			out[i] = v
		case float64:
			out[i] = deliverable.Format(v, t.cols[i].Format, -1)
		}
	}
	return out
}

// split is each cell's lines at its column's width, at most a page of them.
func (t sheet) split(cells []string, style string) [][]string {
	p, st := t.d.PDF(), t.d.Style
	_, ph := p.GetPageSize()
	most := max(int((ph-2*st.Margin-st.Pad)/st.Leading)-2, 1)
	t.d.Font(fonts.Sans, style, st.Body)
	out := make([][]string, len(cells))
	for i, c := range cells {
		out[i] = p.SplitText(paper.Clean(c), t.width-st.Pad)
		if len(out[i]) > most {
			out[i] = out[i][:most]
		}
	}
	return out
}

func (t sheet) height(cells []string, style string) float64 {
	n := 1
	for _, c := range t.split(cells, style) {
		n = max(n, len(c))
	}
	return float64(n)*t.d.Style.Leading + t.d.Style.Pad
}

// row draws one row on the next page when it does not fit, the header first
// when repeat is set and the page turned.
func (t sheet) row(cells []string, style string, repeat bool) {
	p, st := t.d.PDF(), t.d.Style
	h := t.height(cells, style)
	page := p.PageNo()
	t.d.Need(h)
	if t.d.Full() {
		return
	}
	if repeat && p.PageNo() != page {
		t.row(t.header, "B", false)
	}
	parts := t.split(cells, style)
	left, _, _, _ := p.GetMargins()
	y := p.GetY()
	t.d.Ink(st.Ink)
	for i, lines := range parts {
		for j, ln := range lines {
			p.SetXY(left+float64(i)*t.width+st.Pad/2, y+st.Pad/2+float64(j)*st.Leading)
			p.CellFormat(t.width-st.Pad, st.Leading, ln, "", 0, align(t.cols[i].Align), false, 0, "")
		}
	}
	p.SetXY(left, y+h)
	t.d.Rule()
}

func align(a string) string {
	switch a {
	case "right":
		return "R"
	case "center":
		return "C"
	}
	return "L"
}
