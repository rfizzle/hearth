package chart

import (
	"fmt"
	"math"
	"strconv"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// Kind names a primitive.
type Kind string

// The primitives a chart is drawn from.
const (
	Rect     Kind = "rect"
	Line     Kind = "line"
	Polyline Kind = "polyline"
	Wedge    Kind = "wedge"
	Circle   Kind = "circle"
	Text     Kind = "text"
)

// Anchor says which end of a text sits at its x.
type Anchor string

// The three anchors.
const (
	AnchorStart  Anchor = "start"
	AnchorMiddle Anchor = "middle"
	AnchorEnd    Anchor = "end"
)

// Fill and stroke names besides the palette ids "chart.1" to "chart.6".
const (
	Ink  = "ink"
	Grid = "grid"
	None = "none"
)

// Space of a drawing, in units.
const (
	Width     = 600.0
	Height    = 320.0
	PieHeight = 360.0
)

const (
	maxSeries  = 6
	maxPoints  = 52
	maxSlices  = 8
	labelLimit = 12
	thinTo     = 12
	pointsTo   = 26
	barGap     = 0.2
	textSize   = 11.0
)

// Point is a vertex of a polyline.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Item is one primitive; the fields a kind uses are the only ones set.
// Rect uses X, Y, W, H. Line uses X, Y, X2, Y2 and Width. Polyline uses Points
// and Width. Wedge uses X, Y (the centre), R and Start, End as degrees
// clockwise from twelve o'clock. Circle uses X, Y and R. Text uses X, Y (the
// baseline), Text, Size and Anchor.
type Item struct {
	Kind   Kind    `json:"kind"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	X2     float64 `json:"x2,omitempty"`
	Y2     float64 `json:"y2,omitempty"`
	R      float64 `json:"r,omitempty"`
	Start  float64 `json:"start,omitempty"`
	End    float64 `json:"end,omitempty"`
	Points []Point `json:"points,omitempty"`
	// Fill and Stroke are a palette id ("chart.1"), Ink, Grid or None.
	Fill   string  `json:"fill,omitempty"`
	Stroke string  `json:"stroke,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Text   string  `json:"text,omitempty"`
	Size   float64 `json:"size,omitempty"`
	Anchor Anchor  `json:"anchor,omitempty"`
	// Title is the tooltip of a mark: its label and formatted value.
	Title string `json:"title,omitempty"`
}

// Drawing is the items in paint order inside a W by H space.
type Drawing struct {
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Items []Item  `json:"items"`
}

// PaletteID is the id of the hue for the series at index i; past six they
// repeat.
func PaletteID(i int) string {
	return "chart." + strconv.Itoa(i%len(PaletteLight)+1)
}

// round2 is the one rounding every coordinate passes through.
func round2(v float64) float64 {
	r := math.Round(v*100) / 100
	if r == 0 {
		return 0
	}
	return r
}

// Layout draws a line, bar or pie widget.
func Layout(w deliverable.Widget) (Drawing, error) {
	switch w.Kind {
	case deliverable.Line, deliverable.Bar:
		return cartesian(w)
	case deliverable.Pie:
		return pie(w)
	}
	return Drawing{}, fmt.Errorf("chart: a %q widget is not a chart", w.Kind)
}

// cut shortens a label to twelve characters, the last an ellipsis.
func cut(s string) string {
	r := []rune(s)
	if len(r) <= labelLimit {
		return s
	}
	return string(r[:labelLimit-1]) + "…"
}

func text(x, y float64, s string, a Anchor) Item {
	return Item{Kind: Text, X: round2(x), Y: round2(y), Text: s, Size: textSize, Anchor: a, Fill: Ink}
}

// legend lays names out in rows of perRow slots, the last row at the bottom.
func legend(d *Drawing, names []string, perRow int) {
	rows := (len(names) + perRow - 1) / perRow
	slot := (Width - 32) / float64(perRow)
	for i, n := range names {
		x := 16 + float64(i%perRow)*slot
		y := d.H - 16 - float64(rows-1-i/perRow)*24
		d.Items = append(d.Items,
			Item{Kind: Rect, X: round2(x), Y: round2(y - 9), W: 10, H: 10, Fill: PaletteID(i), Stroke: None},
			text(x+14, y, cut(n), AnchorStart))
	}
}

// cartesian lays out a line or bar chart.
func cartesian(w deliverable.Widget) (Drawing, error) {
	n := len(w.Categories)
	if n < 1 || n > maxPoints || len(w.Series) < 1 || len(w.Series) > maxSeries {
		return Drawing{}, fmt.Errorf("chart: %d categories and %d series are outside 1 to %d and 1 to %d", n, len(w.Series), maxPoints, maxSeries)
	}
	for _, s := range w.Series {
		if len(s.Values) != n {
			return Drawing{}, fmt.Errorf("chart: series %q has %d values for %d categories", s.Name, len(s.Values), n)
		}
		for _, v := range s.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return Drawing{}, fmt.Errorf("chart: series %q holds a value that is not a finite number", s.Name)
			}
		}
	}
	stacked := w.Kind == deliverable.Bar && w.Stacked
	lo, hi := dataRange(w.Series, stacked)
	if hi >= 0 {
		lo = math.Min(lo, 0)
	}
	if w.YMin != nil {
		lo = *w.YMin
	}
	if w.YMax != nil {
		hi = *w.YMax
	}
	sc := niceScale(lo, hi, w.YMin != nil, w.YMax != nil)

	const left, right, top = 64.0, 16.0, 24.0
	bottom := Height - 24
	if len(w.Series) > 1 {
		bottom -= 24
	}
	d := Drawing{W: Width, H: Height}
	plotH := bottom - top
	y := func(v float64) float64 {
		v = math.Min(math.Max(v, sc.lo), sc.hi)
		return bottom - (v-sc.lo)/(sc.hi-sc.lo)*plotH
	}
	if w.YLabel != "" {
		d.Items = append(d.Items, text(left, 12, cut(w.YLabel), AnchorStart))
	}
	for _, t := range sc.ticks {
		ty := y(t)
		d.Items = append(d.Items,
			Item{Kind: Line, X: left, Y: round2(ty), X2: Width - right, Y2: round2(ty), Stroke: Grid, Width: 1},
			text(left-8, ty+4, deliverable.Format(t, w.Format, -1), AnchorEnd))
	}
	base := y(0)
	d.Items = append(d.Items, Item{Kind: Line, X: left, Y: round2(base), X2: Width - right, Y2: round2(base), Stroke: Ink, Width: 1})

	step := (n + thinTo - 1) / thinTo
	slot := (Width - left - right) / float64(n)
	for i, c := range w.Categories {
		if i%step == 0 {
			d.Items = append(d.Items, text(left+(float64(i)+0.5)*slot, bottom+16, cut(c), AnchorMiddle))
		}
	}
	if w.Kind == deliverable.Line {
		lines(&d, w, left, slot, y)
	} else {
		bars(&d, w, stacked, left, slot, y)
	}
	if len(w.Series) > 1 {
		names := make([]string, len(w.Series))
		for i, s := range w.Series {
			names[i] = s.Name
		}
		legend(&d, names, len(names))
	}
	return d, nil
}

func lines(d *Drawing, w deliverable.Widget, left, slot float64, y func(float64) float64) {
	for i, s := range w.Series {
		pts := make([]Point, len(s.Values))
		for j, v := range s.Values {
			pts[j] = Point{X: round2(left + (float64(j)+0.5)*slot), Y: round2(y(v))}
		}
		d.Items = append(d.Items, Item{Kind: Polyline, Points: pts, Stroke: PaletteID(i), Fill: None, Width: 2})
		if len(pts) <= pointsTo {
			for j, p := range pts {
				d.Items = append(d.Items, Item{Kind: Circle, X: p.X, Y: p.Y, R: 3, Fill: PaletteID(i), Stroke: None,
					Title: markTitle(w, s.Name, j, s.Values[j])})
			}
		}
	}
}

// bars draws each category's group centred in its slot with a gap of 0.2 of
// the slot; stacked series share one bar, positives up and negatives down.
func bars(d *Drawing, w deliverable.Widget, stacked bool, left, slot float64, y func(float64) float64) {
	series := w.Series
	region := slot * (1 - barGap)
	each := region / float64(len(series))
	if stacked {
		each = region
	}
	rect := func(x, from, to float64, i, c int) {
		top, bot := math.Min(y(from), y(to)), math.Max(y(from), y(to))
		if round2(bot-top) == 0 {
			return
		}
		d.Items = append(d.Items, Item{Kind: Rect, X: round2(x), Y: round2(top), W: round2(each), H: round2(bot - top), Fill: PaletteID(i), Stroke: None,
			Title: markTitle(w, series[i].Name, c, series[i].Values[c])})
	}
	for c := range series[0].Values {
		x0 := left + float64(c)*slot + (slot-region)/2
		up, down := 0.0, 0.0
		for i, s := range series {
			v := s.Values[c]
			switch {
			case !stacked:
				rect(x0+float64(i)*each, 0, v, i, c)
			case v >= 0:
				rect(x0, up, up+v, i, c)
				up += v
			default:
				rect(x0, down+v, down, i, c)
				down += v
			}
		}
	}
}

// dataRange is the lowest and highest value drawn; stacked bars stack.
func dataRange(series []deliverable.Series, stacked bool) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for c := range series[0].Values {
		up, down := 0.0, 0.0
		for _, s := range series {
			v := s.Values[c]
			switch {
			case !stacked:
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			case v >= 0:
				up += v
			default:
				down += v
			}
		}
		if stacked {
			lo, hi = math.Min(lo, down), math.Max(hi, up)
		}
	}
	return lo, hi
}

type scale struct {
	lo, hi float64
	ticks  []float64
}

// niceScale picks the smallest 1, 2 or 5 times a power of ten whose ticks over
// the range number six or fewer; the axis grows out to a tick unless an end is
// fixed.
func niceScale(lo, hi float64, fixLo, fixHi bool) scale {
	if hi <= lo {
		hi = lo + 1
	}
	e := int(math.Floor(math.Log10(hi - lo)))
	const eps = 1e-9
	var sc scale
	for exp := e - 1; exp <= e+1; exp++ {
		for _, m := range [...]float64{1, 2, 5} {
			step := m * math.Pow10(exp)
			nlo, nhi := lo, hi
			if !fixLo {
				nlo = math.Floor(lo/step+eps) * step
			}
			if !fixHi {
				nhi = math.Ceil(hi/step-eps) * step
			}
			k0, k1 := math.Ceil(nlo/step-eps), math.Floor(nhi/step+eps)
			if k1-k0+1 > 6 && !(exp == e+1 && m == 5) {
				continue
			}
			sc = scale{lo: clean(nlo), hi: clean(nhi)}
			for k := k0; k <= k1; k++ {
				sc.ticks = append(sc.ticks, clean(k*step))
			}
			return sc
		}
	}
	return sc
}

// clean drops the float noise of a product such as 3 * 0.1.
func clean(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 12, 64), 64)
	if r == 0 {
		return 0
	}
	return r
}

// markTitle is the tooltip of one value: "Series, category: value", with the
// series left out when the chart has only one.
func markTitle(w deliverable.Widget, series string, c int, v float64) string {
	t := cut(w.Categories[c]) + ": " + deliverable.Format(v, w.Format, -1)
	if len(w.Series) > 1 {
		t = cut(series) + ", " + t
	}
	return t
}
