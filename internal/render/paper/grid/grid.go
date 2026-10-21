package grid

import (
	"context"
	"errors"
	"time"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/chart"
	"github.com/rfizzle/hearth/internal/render/fonts"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// Options are what a render is handed beside the spec.
type Options struct {
	// Paper is the sheet; empty is Letter.
	Paper paper.Paper
	// ModTime is the spec file's modification time: the PDF's dates.
	ModTime time.Time
}

// MaxPages is the most pages a deliverable is drawn on.
const MaxPages = 60

// ErrTooLong is a deliverable that would run past MaxPages.
var ErrTooLong = errors.New("the deliverable renders to more than 60 pages; shorten the tables")

// The grid's measures, in points.
const (
	gutter = 12.0 // between columns, and between rows of cards
	pad    = 12.0 // inside a card
	radius = 6.0  // a card's corners
	// keep is the least of a card taller than a page that is begun at the
	// foot of one; with less room it starts on the next.
	keep = 96.0
	// scratchPages is how many pages a scratch document collects before a
	// fresh one replaces it, well inside paper's own cap.
	scratchPages = 100
)

// Render draws s as a PDF in the print style: a report portrait, a dashboard
// landscape. The same spec and options give the same bytes. A deliverable
// longer than MaxPages is ErrTooLong; one that takes past paper's deadline
// returns the context's error; a fault in the drawing library is
// paper.ErrRender.
// See docs/decisions.md for D45.
func Render(ctx context.Context, s deliverable.Spec, o Options) (out []byte, rep paper.Report, err error) {
	ctx, cancel := context.WithTimeout(ctx, paper.Deadline)
	defer cancel()
	defer func() {
		if recover() != nil {
			out, rep, err = nil, paper.Report{}, paper.ErrRender
		}
	}()
	l := &layout{ctx: ctx, kind: s.Kind, sheet: o.Paper, landscape: s.Kind == deliverable.Dashboard}
	if l.d, err = paper.New(o.Paper, l.landscape, paper.Print()); err != nil {
		return nil, paper.Report{}, err
	}
	l.d.Limit(MaxPages)
	l.header(s)
	for _, sec := range s.Sections {
		if l.stopped() {
			break
		}
		if err := l.section(sec); err != nil {
			return nil, paper.Report{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, paper.Report{}, err
	}
	if l.d.Full() {
		return nil, paper.Report{}, ErrTooLong
	}
	out, pages, err := l.d.Finish(s.Title, o.ModTime)
	if err != nil {
		return nil, paper.Report{}, err
	}
	return out, paper.Report{Pages: pages, Bytes: len(out)}, nil
}

// layout is one render: the document, and a scratch document rows are
// measured on.
type layout struct {
	ctx       context.Context
	d         *paper.Document
	scratch   *paper.Document
	kind      string
	sheet     paper.Paper
	landscape bool
}

// card is one widget placed in a section: its offset from the left margin,
// its width and its measured height, all of the card's outside edge.
type card struct {
	w        deliverable.Widget
	span     int
	x, width float64
	height   float64
	drawing  *chart.Drawing
}

func (l *layout) stopped() bool { return l.ctx.Err() != nil || l.d.Full() }

// content is the height between the top and bottom margins.
func (l *layout) content() float64 {
	_, ph := l.d.PDF().GetPageSize()
	return ph - 2*l.d.Style.Margin
}

// header is the title and the subtitle.
func (l *layout) header(s deliverable.Spec) {
	d, st := l.d, l.d.Style
	d.Font(fonts.Sans, "B", st.Headings[0])
	d.Ink(st.Ink)
	lines(d, s.Title, st.Headings[0]*1.2, "L")
	if s.Subtitle != "" {
		d.Font(fonts.Sans, "", st.Headings[2])
		d.Ink(st.Secondary)
		lines(d, s.Subtitle, st.Headings[2]*1.3, "L")
	}
	d.Space(2 * st.Gap)
}

// section draws a heading, kept with the start of its first row, and the
// rows of its grid.
func (l *layout) section(sec deliverable.Section) error {
	d, st := l.d, l.d.Style
	cols := columns(l.kind, sec)
	unit := (l.d.Width() - float64(cols-1)*gutter) / float64(cols)
	cards, err := l.place(sec, cols, unit)
	if err != nil {
		return err
	}
	rows := pack(cards, cols, unit, l.content())
	if sec.Heading != "" {
		size := st.Headings[2]
		d.Space(2 * st.Gap)
		// The heading goes with its first row, or with the start of a row
		// taller than a page.
		first := 0.0
		if len(rows) > 0 {
			if first = height(rows[0]); first > l.content() {
				first = keep
			}
		}
		if d.PDF().GetY() > st.Margin {
			d.Need(size*1.3 + st.Gap + first + st.Leading)
		}
		d.Font(fonts.Sans, "B", size)
		d.Ink(st.Ink)
		lines(d, sec.Heading, size*1.3, "L")
		d.Space(st.Gap)
	}
	for _, r := range rows {
		if l.stopped() {
			break
		}
		if err := l.row(r); err != nil {
			return err
		}
	}
	return l.ctx.Err()
}

// place sizes each widget's card on the section's grid and measures it.
func (l *layout) place(sec deliverable.Section, cols int, unit float64) ([]card, error) {
	cards := make([]card, len(sec.Widgets))
	for i, w := range sec.Widgets {
		span := min(max(w.Span, 1), cols)
		c := card{w: w, span: span, width: float64(span)*unit + float64(span-1)*gutter}
		if w.Kind == deliverable.Line || w.Kind == deliverable.Bar || w.Kind == deliverable.Pie {
			dr, err := chart.Layout(w)
			if err != nil {
				return nil, err
			}
			c.drawing = &dr
		}
		var err error
		if c.height, err = l.measure(c); err != nil {
			return nil, err
		}
		cards[i] = c
	}
	return cards, l.ctx.Err()
}

// columns is a section's grid width: its own, or the kind's default (a
// dashboard two, a report one).
func columns(kind string, sec deliverable.Section) int {
	if sec.Columns > 0 {
		return sec.Columns
	}
	if kind == deliverable.Dashboard {
		return 2
	}
	return 1
}

// pack lays the cards into rows left to right, a new row when the next card's
// span does not fit beside the last; a card taller than a page has a row to
// itself.
func pack(cards []card, cols int, unit, page float64) [][]card {
	var rows [][]card
	var row []card
	used := 0
	end := func() {
		if len(row) > 0 {
			rows = append(rows, row)
		}
		row, used = nil, 0
	}
	for _, c := range cards {
		if c.height > page {
			end()
			rows = append(rows, []card{c})
			continue
		}
		if used+c.span > cols {
			end()
		}
		c.x = float64(used) * (unit + gutter)
		row = append(row, c)
		used += c.span
	}
	end()
	return rows
}

// height is the tallest card in a row.
func height(row []card) float64 {
	h := 0.0
	for _, c := range row {
		h = max(h, c.height)
	}
	return h
}

// measure draws c on a fresh scratch page, from the top, and returns the
// height of its outside edge; a card that crosses a page measures more than
// a page.
func (l *layout) measure(c card) (float64, error) {
	if l.scratch == nil || l.scratch.PDF().PageCount() > scratchPages {
		s, err := paper.New(l.sheet, l.landscape, paper.Print())
		if err != nil {
			return 0, err
		}
		l.scratch = s
	}
	s, p := l.scratch, l.scratch.PDF()
	p.AddPage()
	m := s.Style.Margin
	first := p.PageNo()
	restore := inside(s, c, m, m+pad)
	err := l.draw(s, c)
	restore()
	_, ph := p.GetPageSize()
	return float64(p.PageNo()-first)*ph + p.GetY() - m + pad, err
}

// inside sets the margins to c's box below y, and the top margin of a page
// it continues on to top, so the widget draws there as on a page of its own.
// It returns what puts the page's margins back.
func inside(d *paper.Document, c card, y, top float64) func() {
	p, m := d.PDF(), d.Style.Margin
	pw, _ := p.GetPageSize()
	left := m + c.x + pad
	p.SetMargins(left, top, pw-(m+c.x+c.width-pad))
	p.SetAutoPageBreak(true, m+pad)
	p.SetXY(left, y+pad)
	return func() {
		p.SetMargins(m, m, m)
		p.SetAutoPageBreak(true, m)
	}
}
