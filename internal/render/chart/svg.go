package chart

import (
	"html"
	"strconv"
	"strings"
)

// SVG writes d as one inline <svg>. It carries a viewBox and no width or
// height, so the page sizes it. Ink and grid are currentColor; a hue is the
// light palette's hex, with a class (c1 to c6) the page may restyle for dark.
// The only input it writes is numbers and escaped text.
func SVG(d Drawing) []byte {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + num(d.W) + " " + num(d.H) + `" role="img" font-family="sans-serif">` + "\n")
	for _, it := range d.Items {
		item(&b, it)
	}
	b.WriteString("</svg>\n")
	return []byte(b.String())
}

// num is the one number formatter: two decimals at most, no trailing zeros.
func num(v float64) string {
	return strconv.FormatFloat(round2(v), 'f', -1, 64)
}

func attr(name string, v float64) string { return " " + name + `="` + num(v) + `"` }

// esc escapes a label for a text node and drops what XML cannot hold.
func esc(s string) string {
	s = strings.ToValidUTF8(s, "�")
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	return html.EscapeString(s)
}

// paint is a fill or stroke attribute: a palette hex, currentColor or none.
// Anything unknown draws as none, never as the string it was given.
func paint(name, id string) string {
	switch id {
	case Ink:
		return " " + name + `="currentColor"`
	case Grid:
		return " " + name + `="currentColor" ` + name + `-opacity="0.15"`
	}
	for i := range PaletteLight {
		if id == PaletteID(i) {
			return " " + name + `="` + PaletteLight[i] + `"`
		}
	}
	return " " + name + `="none"`
}

func class(id string) string {
	for i := range PaletteLight {
		if id == PaletteID(i) {
			return ` class="c` + strconv.Itoa(i+1) + `"`
		}
	}
	return ""
}

// look is the fill, stroke and class an item carries.
func look(it Item) string {
	s := paint("fill", it.Fill) + paint("stroke", it.Stroke)
	if it.Width != 0 {
		s += attr("stroke-width", it.Width)
	}
	if c := class(it.Fill); c != "" {
		return s + c
	}
	return s + class(it.Stroke)
}

func item(b *strings.Builder, it Item) {
	switch it.Kind {
	case Rect:
		b.WriteString("<rect" + attr("x", it.X) + attr("y", it.Y) + attr("width", it.W) + attr("height", it.H) + look(it) + tail(it))
	case Line:
		b.WriteString("<line" + attr("x1", it.X) + attr("y1", it.Y) + attr("x2", it.X2) + attr("y2", it.Y2) + look(it) + tail(it))
	case Polyline:
		pts := make([]string, len(it.Points))
		for i, p := range it.Points {
			pts[i] = num(p.X) + "," + num(p.Y)
		}
		b.WriteString(`<polyline points="` + strings.Join(pts, " ") + `"` + look(it) + ` stroke-linejoin="round"/>` + "\n")
	case Circle:
		b.WriteString("<circle" + attr("cx", it.X) + attr("cy", it.Y) + attr("r", it.R) + look(it) + tail(it))
	case Wedge:
		x1, y1 := OnCircle(it.X, it.Y, it.R, it.Start)
		x2, y2 := OnCircle(it.X, it.Y, it.R, it.End)
		large := "0"
		if it.End-it.Start > 180 {
			large = "1"
		}
		d := "M" + num(it.X) + " " + num(it.Y) + " L" + num(x1) + " " + num(y1) +
			" A" + num(it.R) + " " + num(it.R) + " 0 " + large + " 1 " + num(x2) + " " + num(y2) + " Z"
		b.WriteString(`<path d="` + d + `"` + look(it) + tail(it))
	case Text:
		b.WriteString("<text" + attr("x", it.X) + attr("y", it.Y) + attr("font-size", it.Size) +
			` text-anchor="` + anchor(it.Anchor) + `"` + paint("fill", it.Fill) + ">" + esc(it.Text) + "</text>\n")
	}
}

// anchor maps to one of the three constant words.
func anchor(a Anchor) string {
	switch a {
	case AnchorMiddle:
		return "middle"
	case AnchorEnd:
		return "end"
	}
	return "start"
}

// tail closes a mark: self-closed, or holding its tooltip as a <title>.
func tail(it Item) string {
	if it.Title == "" {
		return "/>\n"
	}
	return "><title>" + esc(it.Title) + "</title></" + tagOf(it.Kind) + ">\n"
}

func tagOf(k Kind) string {
	switch k {
	case Rect:
		return "rect"
	case Circle:
		return "circle"
	}
	return "path"
}
