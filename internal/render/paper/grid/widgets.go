package grid

import (
	"strconv"
	"strings"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/fonts"
	"github.com/rfizzle/hearth/internal/render/paper"
)

// toned is a tone's ink and the fill behind it, the light pair of the
// design's tokens; neutral is the secondary text on the secondary surface.
type toned struct{ ink, fill paper.Colour }

// tone is the colours of a tone's name; anything else is neutral. The word
// always travels with the colour.
// See docs/design.md#colour.
func tone(name string) toned {
	switch name {
	case "positive":
		return toned{hex(0x1F7A3F), hex(0xDDF4E4)}
	case "attention":
		return toned{hex(0x8A5A00), hex(0xFFF1C9)}
	case "negative":
		return toned{hex(0xB3261E), hex(0xFBE4E2)}
	}
	return toned{hex(0x5E5E63), hex(0xE9E9EB)}
}

// hex is a colour written as 0xRRGGBB.
func hex(v uint64) paper.Colour {
	return paper.Colour{R: int(v >> 16 & 0xFF), G: int(v >> 8 & 0xFF), B: int(v & 0xFF)}
}

// draw draws the widget of c inside the current margins from the current
// height.
func (l *layout) draw(d *paper.Document, c card) error {
	w := c.w
	switch w.Kind {
	case deliverable.Metric:
		metric(d, w)
	case deliverable.Progress:
		progress(d, w)
	case deliverable.Light:
		light(d, w)
	case deliverable.Line, deliverable.Bar, deliverable.Pie:
		title(d, w.Title, d.Style.Ink)
		if c.drawing != nil {
			draw(d, *c.drawing)
		}
	case deliverable.Table:
		title(d, w.Title, d.Style.Ink)
		table(l, d, w)
	case deliverable.List:
		title(d, w.Title, d.Style.Ink)
		list(l, d, w)
	case deliverable.Callout:
		title(d, w.Title, tone(w.Tone).ink)
		return d.Markdown(l.ctx, []byte(w.Markdown))
	case deliverable.Text:
		return d.Markdown(l.ctx, []byte(w.Markdown))
	}
	return nil
}

// title is a card's heading in bold, if it has one.
func title(d *paper.Document, s string, ink paper.Colour) {
	if s == "" {
		return
	}
	st := d.Style
	d.Font(fonts.Sans, "B", st.Body)
	d.Ink(ink)
	lines(d, s, st.Leading, "L")
	d.Space(st.Gap / 2)
}

// say is s in secondary ink at size, wrapped, if there is any.
func say(d *paper.Document, s string, size float64) {
	if s == "" {
		return
	}
	d.Font(fonts.Sans, "", size)
	d.Ink(d.Style.Secondary)
	lines(d, s, size*1.35, "L")
}

// notes are the caption and, on a bound widget, the time its value was read.
func notes(d *paper.Document, w deliverable.Widget) {
	say(d, w.Caption, d.Style.Caption+1)
	if w.AsOf != "" {
		at := w.AsOf
		if t, ok := strings.CutSuffix(at, "Z"); ok {
			at = strings.Replace(t, "T", " ", 1) + " UTC"
		}
		say(d, "As of "+at, d.Style.Caption)
	}
}

// decimalsOf is a widget's decimals, or -1 for the format's own.
func decimalsOf(w deliverable.Widget) int {
	if w.Decimals != nil {
		return *w.Decimals
	}
	return -1
}

func metric(d *paper.Document, w deliverable.Widget) {
	p, st := d.PDF(), d.Style
	say(d, w.Label, st.Body)
	shown := "—"
	if w.Value != nil {
		shown = deliverable.Format(*w.Value, w.Format, decimalsOf(w))
	}
	big := st.Headings[0]
	d.Need(big * 1.25)
	d.Font(fonts.Sans, "B", big)
	d.Ink(st.Ink)
	shown = paper.Clean(shown)
	p.CellFormat(p.GetStringWidth(shown), big*1.25, shown, "", 0, "L", false, 0, "")
	if w.Unit != "" {
		d.Font(fonts.Sans, "", st.Headings[2])
		d.Ink(st.Secondary)
		p.CellFormat(d.Width()-p.GetStringWidth(shown), big*1.25, " "+paper.Clean(w.Unit), "", 0, "L", false, 0, "")
	}
	p.Ln(big * 1.25)
	if w.Delta != nil {
		change := deliverable.Format(w.Delta.Value, w.Format, decimalsOf(w))
		if w.Delta.Value > 0 {
			change = "+" + change
		}
		ink := tone("").ink
		if w.Delta.Good != nil {
			ink = tone("negative").ink
			if *w.Delta.Good {
				ink = tone("positive").ink
			}
		}
		d.Font(fonts.Sans, "B", st.Body)
		d.Ink(ink)
		lines(d, change, st.Leading, "L")
	}
	notes(d, w)
}

func progress(d *paper.Document, w deliverable.Widget) {
	p, st := d.PDF(), d.Style
	say(d, w.Label, st.Body)
	share, shown := 0.0, "—"
	if w.Done != nil && w.Total != nil {
		shown = deliverable.Format(*w.Done, "number", -1) + " of " + deliverable.Format(*w.Total, "number", -1)
		if *w.Total > 0 {
			share = min(max(*w.Done / *w.Total, 0), 1)
		}
	}
	d.Font(fonts.Sans, "B", st.Body)
	d.Ink(st.Ink)
	lines(d, shown, st.Leading, "L")
	bar := 6.0
	d.Need(bar + st.Gap)
	x, y, width := p.GetX(), p.GetY()+st.Gap/2, d.Width()
	track, done := tone("").fill, tone("positive").ink
	p.SetFillColor(track.R, track.G, track.B)
	p.RoundedRect(x, y, width, bar, bar/2, "1234", "F")
	if share > 0 {
		p.SetFillColor(done.R, done.G, done.B)
		p.RoundedRect(x, y, max(width*share, bar), bar, bar/2, "1234", "F")
	}
	p.SetXY(x, y+bar+st.Gap/2)
	notes(d, w)
}

func light(d *paper.Document, w deliverable.Widget) {
	p, st := d.PDF(), d.Style
	say(d, w.Label, st.Body)
	word := w.Word
	if word == "" {
		word = "Not read yet"
	}
	ink := tone(w.Tone).ink
	d.Need(st.Leading + st.Gap)
	x, y := p.GetX(), p.GetY()+st.Gap/2
	r := st.Body * 0.3
	p.SetFillColor(ink.R, ink.G, ink.B)
	p.Circle(x+r, y+st.Leading/2, r, "F")
	p.SetXY(x+3*r, y)
	d.Font(fonts.Sans, "B", st.Body)
	d.Ink(ink)
	p.CellFormat(d.Width()-3*r, st.Leading, paper.Clean(word), "", 1, "L", false, 0, "")
	p.SetY(p.GetY() + st.Gap/2)
	notes(d, w)
}

// list draws bullets, numbers or boxes, a done item struck through in
// secondary ink, and each item's detail below it.
func list(l *layout, d *paper.Document, w deliverable.Widget) {
	p, st := d.PDF(), d.Style
	left, _, _, _ := p.GetMargins()
	for i, it := range w.Items {
		if l.stopped() {
			return
		}
		d.Need(st.Leading)
		y := p.GetY()
		d.Font(fonts.Sans, "", st.Body)
		d.Ink(st.Ink)
		switch w.Style {
		case "numbered":
			p.CellFormat(st.Indent, st.Leading, strconv.Itoa(i+1)+".", "", 0, "L", false, 0, "")
		case "check":
			box(d, left, y, it.Done)
		default:
			p.CellFormat(st.Indent, st.Leading, "•", "", 0, "L", false, 0, "")
		}
		p.SetLeftMargin(left + st.Indent)
		p.SetXY(left+st.Indent, y)
		style, ink := "", st.Ink
		if it.Done {
			style, ink = "S", st.Secondary
		}
		d.Font(fonts.Sans, style, st.Body)
		d.Ink(ink)
		lines(d, it.Text, st.Leading, "L")
		say(d, it.Detail, st.Caption+1)
		p.SetLeftMargin(left)
		p.SetX(left)
	}
}

// box is a task's box at x on the line at y, ticked when done.
func box(d *paper.Document, x, y float64, done bool) {
	p, st := d.PDF(), d.Style
	side := st.Body * 0.7
	y += (st.Leading - side) / 2
	line(d, st.Secondary)
	p.SetLineWidth(st.Hairline * 1.5)
	p.Rect(x, y, side, side, "D")
	if done {
		p.Line(x+side*0.2, y+side*0.55, x+side*0.45, y+side*0.8)
		p.Line(x+side*0.45, y+side*0.8, x+side*0.85, y+side*0.2)
	}
}
