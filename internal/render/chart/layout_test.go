package chart

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// update rewrites the goldens: go test ./internal/render/chart -run TestGoldens -update
var update = flag.Bool("update", false, "rewrite the golden files under testdata")

func ptr(v float64) *float64 { return &v }

func cats(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("Week %d", i+1)
	}
	return out
}

func wave(n int, scale, shift float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64((i*7+int(shift))%11)*scale + shift
	}
	return out
}

type fixture struct {
	name string
	w    deliverable.Widget
}

func fixtures() []fixture {
	six := make([]deliverable.Series, 6)
	for i := range six {
		six[i] = deliverable.Series{Name: fmt.Sprintf("Series number %d", i+1), Values: wave(52, 1.5, float64(i*3))}
	}
	mixed := []deliverable.Series{
		{Name: "Revenue", Values: []float64{120, -40, 75.5, 200}},
		{Name: "Costs", Values: []float64{-60, 30, -90, 45.25}},
		{Name: "Refunds", Values: []float64{10, -5, 0, -20}},
	}
	return []fixture{
		{"line-one", deliverable.Widget{Kind: deliverable.Line, Title: "Runs", YLabel: "Runs", Categories: []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
			Series: []deliverable.Series{{Name: "Runs", Values: []float64{3, 8, 5, 13, 9}}}}},
		{"line-six", deliverable.Widget{Kind: deliverable.Line, Title: "Six", Categories: cats(52), Series: six}},
		{"bar-grouped", deliverable.Widget{Kind: deliverable.Bar, Title: "Grouped", Categories: []string{"Q1", "Q2", "Q3", "A rather long quarter"}, Series: mixed}},
		{"bar-stacked", deliverable.Widget{Kind: deliverable.Bar, Title: "Stacked", Stacked: true, Categories: []string{"Q1", "Q2", "Q3", "Q4"}, Series: mixed}},
		{"pie-two", deliverable.Widget{Kind: deliverable.Pie, Title: "Two", Slices: []deliverable.Slice{{Label: "Done", Value: 7}, {Label: "Open", Value: 3}}}},
		{"pie-eight", deliverable.Widget{Kind: deliverable.Pie, Title: "Eight", Slices: []deliverable.Slice{
			{Label: "Alpha", Value: 30}, {Label: "Beta", Value: 20}, {Label: "Gamma", Value: 15}, {Label: "Delta", Value: 12},
			{Label: "Epsilon", Value: 10}, {Label: "Zeta", Value: 8}, {Label: "Eta", Value: 3}, {Label: "Theta is a long label", Value: 2}}}},
	}
}

func TestGoldens(t *testing.T) {
	for _, f := range fixtures() {
		t.Run(f.name, func(t *testing.T) {
			d, err := Layout(f.w)
			if err != nil {
				t.Fatal(err)
			}
			layout, err := json.MarshalIndent(d, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			again, _ := Layout(f.w)
			if got := SVG(again); !bytes.Equal(got, SVG(d)) {
				t.Fatal("two layouts of one widget wrote different SVG")
			}
			check(t, f.name+".layout.json", append(layout, '\n'))
			check(t, f.name+".svg", SVG(d))
		})
	}
}

func check(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden (run with -update): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from its golden; run with -update and read the diff", name)
	}
}

func TestNiceTicks(t *testing.T) {
	for _, c := range []struct {
		lo, hi float64
		want   []float64
	}{
		{0, 13, []float64{0, 5, 10, 15}},
		{0, 100, []float64{0, 20, 40, 60, 80, 100}},
		{-40, 200, []float64{-50, 0, 50, 100, 150, 200}},
		{0, 0.3, []float64{0, 0.1, 0.2, 0.3}},
		{-9, -2, []float64{-10, -8, -6, -4, -2}},
		{5, 5, []float64{5, 5.2, 5.4, 5.6, 5.8, 6}},
	} {
		got := niceScale(c.lo, c.hi, false, false).ticks
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("niceScale(%v, %v) ticks = %v, want %v", c.lo, c.hi, got, c.want)
		}
	}
	for hi := 1.0; hi < 5000; hi *= 1.37 {
		for _, lo := range []float64{0, -hi / 3} {
			if n := len(niceScale(lo, hi, false, false).ticks); n < 4 || n > 6 {
				t.Errorf("niceScale(%v, %v) has %d ticks, want 4 to 6", lo, hi, n)
			}
		}
	}
	fixed := niceScale(0, 7, true, true)
	if fixed.lo != 0 || fixed.hi != 7 {
		t.Errorf("fixed ends moved: %v to %v", fixed.lo, fixed.hi)
	}
}

func TestRules(t *testing.T) {
	texts := func(d Drawing) []string {
		var out []string
		for _, it := range d.Items {
			if it.Kind == Text {
				out = append(out, it.Text)
			}
		}
		return out
	}
	count := func(d Drawing, k Kind) (n int) {
		for _, it := range d.Items {
			if it.Kind == k {
				n++
			}
		}
		return n
	}
	byName := map[string]Drawing{}
	for _, f := range fixtures() {
		d, err := Layout(f.w)
		if err != nil {
			t.Fatal(err)
		}
		byName[f.name] = d
	}
	// 52 categories thin to every fifth label: 11 of them, plus ticks and legend.
	six := byName["line-six"]
	if got := count(six, Circle); got != 0 {
		t.Errorf("52 points drew %d markers, want none", got)
	}
	if got := count(six, Polyline); got != 6 {
		t.Errorf("six series drew %d polylines", got)
	}
	if !contains(texts(six), "Week 1") || contains(texts(six), "Week 2") || !contains(texts(six), "Week 6") {
		t.Errorf("52 labels were not thinned to every 5th: %v", texts(six))
	}
	if !contains(texts(six), "Series numb…") {
		t.Errorf("a long legend name was not cut at 12: %v", texts(six))
	}
	if got := count(byName["line-one"], Circle); got != 5 {
		t.Errorf("five points drew %d markers", got)
	}
	if contains(texts(byName["line-one"]), "Runs") == false {
		t.Error("the y label is missing")
	}
	if got := count(byName["line-one"], Rect); got != 0 {
		t.Errorf("one series drew a legend: %d rects", got)
	}
	// Pie: slices of 3 % and 2 % carry no percentage.
	pct := 0
	for _, s := range texts(byName["pie-eight"]) {
		if strings.HasSuffix(s, "%") {
			pct++
		}
	}
	if pct != 6 {
		t.Errorf("eight slices gave %d percentages, want 6 (two are under 4%%)", pct)
	}
	first := byName["pie-two"].Items[0]
	if first.Kind != Wedge || first.Start != 0 || first.End != 252 {
		t.Errorf("first wedge = %+v, want 0 to 252 degrees", first)
	}
	one, _ := Layout(deliverable.Widget{Kind: deliverable.Pie, Slices: []deliverable.Slice{{Label: "A", Value: 4}, {Label: "B"}}})
	if one.Items[0].Kind != Circle {
		t.Errorf("a whole pie is %v, want a circle", one.Items[0].Kind)
	}
	// Stacked negatives sit below the zero line, positives above.
	stack := byName["bar-stacked"]
	var base float64
	for _, it := range stack.Items {
		if it.Kind == Line && it.Stroke == Ink {
			base = it.Y
		}
	}
	for _, it := range stack.Items {
		if it.Kind == Rect && it.W > 10 && it.Y < base && it.Y+it.H > base {
			t.Errorf("a stacked bar straddles the zero line: %+v", it)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestRefusals(t *testing.T) {
	line := func(cats []string, s ...deliverable.Series) deliverable.Widget {
		return deliverable.Widget{Kind: deliverable.Line, Categories: cats, Series: s}
	}
	for name, w := range map[string]deliverable.Widget{
		"table":        {Kind: deliverable.Table},
		"no series":    line([]string{"a"}),
		"short series": line([]string{"a", "b"}, deliverable.Series{Name: "x", Values: []float64{1}}),
		"one slice":    {Kind: deliverable.Pie, Slices: []deliverable.Slice{{Label: "a", Value: 1}}},
		"zero pie":     {Kind: deliverable.Pie, Slices: []deliverable.Slice{{Label: "a"}, {Label: "b"}}},
		"negative pie": {Kind: deliverable.Pie, Slices: []deliverable.Slice{{Label: "a", Value: 2}, {Label: "b", Value: -1}}},
		"seven series": line([]string{"a"}, make([]deliverable.Series, 7)...),
		"fifty-three":  line(cats(53), deliverable.Series{Name: "x", Values: make([]float64, 53)}),
		"nine slices":  {Kind: deliverable.Pie, Slices: make([]deliverable.Slice, 9)},
		"not a number": line([]string{"a"}, deliverable.Series{Name: "x", Values: []float64{nan()}}),
	} {
		if _, err := Layout(w); err == nil {
			t.Errorf("%s: Layout drew it", name)
		}
	}
}

func nan() float64 { z := 0.0; return z / z }

func TestPaletteByOrder(t *testing.T) {
	d, _ := Layout(fixtures()[1].w)
	var ids []string
	for _, it := range d.Items {
		if it.Kind == Polyline {
			ids = append(ids, it.Stroke)
		}
	}
	if got := strings.Join(ids, " "); got != "chart.1 chart.2 chart.3 chart.4 chart.5 chart.6" {
		t.Errorf("series hues = %s", got)
	}
}

func TestFixedAxisClamps(t *testing.T) {
	w := deliverable.Widget{Kind: deliverable.Line, YMin: ptr(0), YMax: ptr(10), Categories: []string{"a", "b"},
		Series: []deliverable.Series{{Name: "x", Values: []float64{5, 50}}}}
	d, err := Layout(w)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range d.Items {
		if it.Kind == Polyline && it.Points[1].Y != 24 {
			t.Errorf("a value above y_max sits at %v, want the top edge 24", it.Points[1].Y)
		}
	}
}
