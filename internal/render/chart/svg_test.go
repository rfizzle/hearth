package chart

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/deliverable"
)

var onAttr = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)

func TestSVGIsInert(t *testing.T) {
	evil := `<svg onload=alert(1)>"&'`
	widgets := append(fixtures(),
		fixture{"evil-line", deliverable.Widget{Kind: deliverable.Line, YLabel: evil, Categories: []string{evil, "b"},
			Series: []deliverable.Series{{Name: evil, Values: []float64{1, 2}}, {Name: "b\x00\x1b", Values: []float64{2, 1}}}}},
		fixture{"evil-pie", deliverable.Widget{Kind: deliverable.Pie, Slices: []deliverable.Slice{{Label: evil, Value: 1}, {Label: "b", Value: 1}}}},
	)
	for _, f := range widgets {
		d, err := Layout(f.w)
		if err != nil {
			t.Fatal(err)
		}
		out := string(SVG(d))
		for _, bad := range []string{"<script", "href", "<image", "<foreignObject", "xlink", "style=", "javascript:", "<a ", "<svg onload", "\x00", "\x1b"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s: SVG holds %q", f.name, bad)
			}
		}
		if onAttr.MatchString(out) {
			t.Errorf("%s: SVG holds an on-attribute", f.name)
		}
		if !strings.HasPrefix(out, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 `) {
			t.Errorf("%s: the SVG does not open with a viewBox: %.80s", f.name, out)
		}
		if strings.Contains(strings.SplitN(out, ">", 2)[0], " width=") {
			t.Errorf("%s: the root carries a width", f.name)
		}
	}
	d, _ := Layout(widgets[len(widgets)-2].w)
	if out := string(SVG(d)); !strings.Contains(out, "&lt;svg onload…") {
		t.Errorf("a hostile label was not escaped: %s", out)
	}
}

func TestSVGOnlyWritesKnownPaint(t *testing.T) {
	d := Drawing{W: 600, H: 320, Items: []Item{{Kind: Rect, W: 1, H: 1, Fill: `red" onclick="x`, Stroke: "#fff"}}}
	out := string(SVG(d))
	if strings.Contains(out, "onclick") || strings.Contains(out, "red") || !strings.Contains(out, `fill="none"`) {
		t.Errorf("an unknown paint leaked: %s", out)
	}
}

func TestNumbers(t *testing.T) {
	for in, want := range map[float64]string{0: "0", 1.006: "1.01", -0.001: "0", 12.3: "12.3", 100: "100", 1.0 / 3: "0.33"} {
		if got := num(in); got != want {
			t.Errorf("num(%v) = %s, want %s", in, got, want)
		}
	}
}

func TestEscapeKeepsQuotesAndAmpersands(t *testing.T) {
	if got := esc("<a href=\"x\">&'\x00"); got != "&lt;a href=&#34;x&#34;&gt;&amp;&#39;" {
		t.Errorf("esc = %s", got)
	}
}
