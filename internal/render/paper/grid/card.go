package grid

import (
	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// row draws a row of cards side by side at one height, on the next page when
// it does not fit on this one. A row of one card taller than a page is drawn
// by tall.
func (l *layout) row(r []card) error {
	if len(r) == 1 && r[0].height > l.content() {
		return l.tall(r[0])
	}
	d, p := l.d, l.d.PDF()
	h := height(r)
	m := d.Style.Margin
	// A line of slack: a heading near a card's foot keeps a line after it,
	// which the measure, drawn from a page's top, never had to.
	if p.GetY() > m {
		d.Need(h + d.Style.Leading)
	}
	if d.Full() {
		return nil
	}
	y := p.GetY()
	for _, c := range r {
		frame(d, c, m+c.x, y, h)
	}
	for _, c := range r {
		restore := inside(d, c, y, y+pad)
		err := l.draw(d, c)
		restore()
		if err != nil {
			return err
		}
	}
	p.SetXY(m, y+h)
	d.Space(gutter)
	return nil
}

// tall draws a card taller than a page: begun here if a fair part fits,
// continued on the pages after, its edge drawn on each page it crosses once
// its end is known.
func (l *layout) tall(c card) error {
	d, p := l.d, l.d.PDF()
	m := d.Style.Margin
	if p.GetY() > m {
		d.Need(keep)
	}
	if d.Full() {
		return nil
	}
	first, y := p.PageNo(), p.GetY()
	restore := inside(d, c, y, m+pad)
	err := l.draw(d, c)
	restore()
	if err != nil {
		return err
	}
	last := p.PageNo()
	_, ph := p.GetPageSize()
	end := min(p.GetY()+pad, ph-m)
	for page := first; page <= last; page++ {
		p.SetPage(page)
		top, bottom := m, ph-m
		if page == first {
			top = y
		}
		if page == last {
			bottom = end
		}
		edge(d, m+c.x, top, c.width, bottom-top, page == first, page == last)
	}
	p.SetPage(last)
	p.SetXY(m, end)
	d.Space(gutter)
	return nil
}

// frame is a card's edge, a hairline with round corners, filled in its tone
// for a callout.
func frame(d *paper.Document, c card, x, y, h float64) {
	p := d.PDF()
	line(d, d.Style.Rule)
	style := "D"
	if c.w.Kind == deliverable.Callout {
		f := tone(c.w.Tone).fill
		p.SetFillColor(f.R, f.G, f.B)
		style = "FD"
	}
	p.RoundedRect(x, y, c.width, h, radius, "1234", style)
}

// edge is the part of a tall card's edge on one page: its sides, and its top
// and bottom where it opens and closes. It is never filled: the text is
// already drawn.
func edge(d *paper.Document, x, y, w, h float64, opens, closes bool) {
	p := d.PDF()
	line(d, d.Style.Rule)
	p.Line(x, y, x, y+h)
	p.Line(x+w, y, x+w, y+h)
	if opens {
		p.Line(x, y, x+w, y)
	}
	if closes {
		p.Line(x, y+h, x+w, y+h)
	}
}

// line sets the stroke to a hairline in c.
func line(d *paper.Document, c paper.Colour) {
	d.PDF().SetDrawColor(c.R, c.G, c.B)
	d.PDF().SetLineWidth(d.Style.Hairline)
}

// lines draws s across the column, wrapped, each line lead high and aligned
// L, C or R, in the font and ink already set; a line that does not fit goes
// to the next page.
func lines(d *paper.Document, s string, lead float64, align string) {
	p := d.PDF()
	for _, ln := range p.SplitText(paper.Clean(s), d.Width()) {
		if d.Full() {
			return
		}
		d.Need(lead)
		p.CellFormat(d.Width(), lead, ln, "", 1, align, false, 0, "")
	}
}
