package paper

import (
	east "github.com/yuin/goldmark/extension/ast"

	"github.com/rfizzle/hearth/internal/render/fonts"
)

// table draws a table across the column in equal widths: the header row in
// bold, a hairline under every row, each cell's text plain and wrapped. A
// cell too tall for a page is cut to one page.
func (w *walker) table(t *east.Table) {
	p, s := w.d.pdf, w.d.Style
	cols := len(t.Alignments)
	if cols == 0 {
		return
	}
	l, _, _, _ := p.GetMargins()
	_, ph := p.GetPageSize()
	cw := w.d.Width() / float64(cols)
	most := max(int((ph-2*s.Margin-s.Pad)/s.Leading), 1)
	w.d.Ink(s.Ink)
	for row := t.FirstChild(); row != nil && !w.stopped(); row = row.NextSibling() {
		style := ""
		if _, ok := row.(*east.TableHeader); ok {
			style = "B"
		}
		w.d.Font(fonts.Sans, style, s.Body)
		lines := make([][]string, cols)
		height := 1
		i := 0
		for c := row.FirstChild(); c != nil && i < cols; c = c.NextSibling() {
			lines[i] = p.SplitText(plain(c, w.src), cw-s.Pad)
			if len(lines[i]) > most {
				lines[i] = lines[i][:most]
			}
			height = max(height, len(lines[i]))
			i++
		}
		rowH := float64(height)*s.Leading + s.Pad
		w.d.Need(rowH)
		if w.d.Full() {
			return
		}
		y := p.GetY()
		for i, cell := range lines {
			for j, line := range cell {
				p.SetXY(l+float64(i)*cw+s.Pad/2, y+s.Pad/2+float64(j)*s.Leading)
				p.CellFormat(cw-s.Pad, s.Leading, line, "", 0, align(t.Alignments[i]), false, 0, "")
			}
		}
		p.SetXY(l, y+rowH)
		w.d.Rule()
	}
	w.d.Space(s.Gap)
}

func align(a east.Alignment) string {
	switch a {
	case east.AlignRight:
		return "R"
	case east.AlignCenter:
		return "C"
	}
	return "L"
}
