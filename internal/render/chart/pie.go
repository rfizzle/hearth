package chart

import (
	"fmt"
	"math"
	"strconv"

	"github.com/rfizzle/hearth/internal/deliverable"
)

const (
	pieX, pieY, pieR = 300.0, 150.0, 110.0
	minLabelled      = 0.04
)

// pie lays out wedges from twelve o'clock clockwise, the percentage outside a
// wedge of 4 % or more, and a legend of up to four names a row below.
func pie(w deliverable.Widget) (Drawing, error) {
	if len(w.Slices) < 2 || len(w.Slices) > maxSlices {
		return Drawing{}, fmt.Errorf("chart: %d slices are outside 2 to %d", len(w.Slices), maxSlices)
	}
	sum := 0.0
	for _, s := range w.Slices {
		if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) || s.Value < 0 {
			return Drawing{}, fmt.Errorf("chart: slice %q is not a number of zero or more", s.Label)
		}
		sum += s.Value
	}
	if sum <= 0 {
		return Drawing{}, fmt.Errorf("chart: every slice is zero")
	}
	d := Drawing{W: Width, H: PieHeight}
	names := make([]string, len(w.Slices))
	run := 0.0
	for i, s := range w.Slices {
		names[i] = s.Label
		if s.Value == 0 {
			continue
		}
		start := round2(run / sum * 360)
		run += s.Value
		end := round2(run / sum * 360)
		if s.Value == sum {
			d.Items = append(d.Items, Item{Kind: Circle, X: pieX, Y: pieY, R: pieR, Fill: PaletteID(i), Stroke: None})
		} else {
			d.Items = append(d.Items, Item{Kind: Wedge, X: pieX, Y: pieY, R: pieR, Start: start, End: end, Fill: PaletteID(i), Stroke: None})
		}
		if share := s.Value / sum; share >= minLabelled {
			x, y := OnCircle(pieX, pieY, pieR+16, (start+end)/2)
			a := AnchorMiddle
			switch {
			case x > pieX+8:
				a = AnchorStart
			case x < pieX-8:
				a = AnchorEnd
			}
			d.Items = append(d.Items, text(x, y+4, strconv.FormatFloat(math.Round(share*100), 'f', 0, 64)+"%", a))
		}
	}
	legend(&d, names, 4)
	return d, nil
}

// OnCircle is the point at deg degrees clockwise from twelve o'clock on the
// circle about (cx, cy). The products are converted to float64 so no platform
// fuses them into a different last bit.
func OnCircle(cx, cy, r, deg float64) (x, y float64) {
	rad := deg * math.Pi / 180
	return round2(cx + float64(r*math.Sin(rad))), round2(cy - float64(r*math.Cos(rad)))
}
