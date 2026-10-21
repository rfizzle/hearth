package grid

import (
	"strconv"

	"github.com/go-pdf/fpdf"

	"github.com/rfizzle/hearth/internal/render/chart"
	"github.com/rfizzle/hearth/internal/render/fonts"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// chartShare is a chart's greatest height as a share of its card's width.
const chartShare = 0.55

// draw draws a chart's primitives with fpdf's vector calls, scaled to fit
// the column's width and chartShare of it in height, centred: the same items
// the web page's SVG is written from, never a picture of them.
// See docs/decisions.md for D45.
func draw(d *paper.Document, dr chart.Drawing) {
	p := d.PDF()
	width := d.Width()
	k := min(width/dr.W, width*chartShare/dr.H)
	d.Need(dr.H * k)
	left, _, _, _ := p.GetMargins()
	pen := pen{d: d, x: left + (width-dr.W*k)/2, y: p.GetY(), k: k}
	p.SetLineJoinStyle("round")
	p.SetLineCapStyle("round")
	for _, it := range dr.Items {
		pen.item(it)
	}
	p.SetLineJoinStyle("miter")
	p.SetLineCapStyle("butt")
	p.SetXY(left, pen.y+dr.H*k)
}

// pen maps a drawing's units to the page: an origin and a scale.
type pen struct {
	d    *paper.Document
	x, y float64
	k    float64
}

func (n pen) at(x, y float64) (float64, float64) { return n.x + x*n.k, n.y + y*n.k }

// paint sets the fill and stroke an item names and returns fpdf's style for
// them: F, D, FD, or empty when the item paints nothing.
func (n pen) paint(it chart.Item) string {
	p := n.d.PDF()
	style := ""
	if c, ok := n.colour(it.Fill); ok {
		p.SetFillColor(c.R, c.G, c.B)
		style = "F"
	}
	if c, ok := n.colour(it.Stroke); ok {
		p.SetDrawColor(c.R, c.G, c.B)
		w := it.Width
		if w == 0 {
			w = 1
		}
		p.SetLineWidth(w * n.k)
		style += "D"
	}
	return style
}

// colour is a palette id, ink or grid as a colour; anything else paints
// nothing, never the string it was given.
func (n pen) colour(id string) (paper.Colour, bool) {
	switch id {
	case chart.Ink:
		return n.d.Style.Ink, true
	case chart.Grid:
		return n.d.Style.Rule, true
	}
	for i, code := range chart.PaletteLight {
		if id == chart.PaletteID(i) {
			v, err := strconv.ParseUint(code[1:], 16, 32)
			if err != nil {
				return paper.Colour{}, false
			}
			return hex(v), true
		}
	}
	return paper.Colour{}, false
}

func (n pen) item(it chart.Item) {
	if it.Kind == chart.Text {
		n.text(it)
		return
	}
	style := n.paint(it)
	if style == "" {
		return
	}
	p := n.d.PDF()
	x, y := n.at(it.X, it.Y)
	switch it.Kind {
	case chart.Rect:
		p.Rect(x, y, it.W*n.k, it.H*n.k, style)
	case chart.Line:
		x2, y2 := n.at(it.X2, it.Y2)
		p.Line(x, y, x2, y2)
	case chart.Polyline:
		if len(it.Points) < 2 {
			return
		}
		pts := make([]fpdf.PointType, len(it.Points))
		for i, pt := range it.Points {
			pts[i].X, pts[i].Y = n.at(pt.X, pt.Y)
		}
		p.MoveTo(pts[0].X, pts[0].Y)
		for _, pt := range pts[1:] {
			p.LineTo(pt.X, pt.Y)
		}
		p.DrawPath("D")
	case chart.Circle:
		p.Circle(x, y, it.R*n.k, style)
	case chart.Wedge:
		n.wedge(it, style)
	}
}

// wedge is a pie slice: from the centre to the arc's end, back along the arc
// to its start, and closed. The layout's angles run clockwise from twelve
// o'clock on a page whose y runs down; fpdf's run counter-clockwise from
// three o'clock, so the arc is drawn from the end angle to the start.
func (n pen) wedge(it chart.Item, style string) {
	p := n.d.PDF()
	cx, cy := n.at(it.X, it.Y)
	ex, ey := chart.OnCircle(it.X, it.Y, it.R, it.End)
	ex, ey = n.at(ex, ey)
	r := it.R * n.k
	p.MoveTo(cx, cy)
	p.LineTo(ex, ey)
	p.ArcTo(cx, cy, r, r, 0, 90-it.End, 90-it.Start)
	p.ClosePath()
	p.DrawPath(style)
}

// text is a label at its baseline, its anchor's end at x.
func (n pen) text(it chart.Item) {
	p := n.d.PDF()
	ink, ok := n.colour(it.Fill)
	if !ok {
		ink = n.d.Style.Ink
	}
	n.d.Font(fonts.Sans, "", it.Size*n.k)
	n.d.Ink(ink)
	s := paper.Clean(it.Text)
	x, y := n.at(it.X, it.Y)
	switch it.Anchor {
	case chart.AnchorMiddle:
		x -= p.GetStringWidth(s) / 2
	case chart.AnchorEnd:
		x -= p.GetStringWidth(s)
	}
	p.Text(x, y, s)
}
