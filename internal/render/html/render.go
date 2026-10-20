package html

import (
	"embed"
	"html/template"
	"strings"

	"github.com/rfizzle/hearth/internal/deliverable"
	"github.com/rfizzle/hearth/internal/render/chart"
)

// stylesheet is the Tailwind build of the templates' class names, inlined in
// the page's one <style>. It is generated (make tailwind) and committed.
//
//go:embed deliverable.css
var stylesheet string

//go:embed templates/*.html
var templates embed.FS

// page is parsed once. Its funcs are the only way a value reaches the page
// other than a {{.}} action html/template escapes; the three that return
// trusted types are stylesheet, markdown and chart, and nothing else in the
// package converts to a template type.
var page = template.Must(template.New("page").Funcs(template.FuncMap{
	"stylesheet": func() template.CSS { return template.CSS(stylesheet) },
	"markdown":   func(src string) template.HTML { return template.HTML(markdown(src)) },
	"chart":      drawChart,
	"columns":    columns,
	"metric":     metric,
	"delta":      delta,
	"deltaTone":  deltaTone,
	"progress":   progress,
	"percent":    percent,
	"cells":      cells,
	"asOf":       asOf,
}).ParseFS(templates, "templates/*.html"))

// Render writes s as one self-contained web page: no script, no external
// resource, a content-security-policy that forbids both, the stylesheet
// inlined and the charts as inline SVG. The same Spec gives the same bytes
// every time; nothing on the page depends on the clock or the host. s is
// treated as valid (deliverable.Validate has passed); a spec that is not still
// renders inert, or fails with an error, never as a partial page.
func Render(s deliverable.Spec) ([]byte, error) {
	var b strings.Builder
	if err := page.ExecuteTemplate(&b, "page", s); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// drawChart is a line, bar or pie's SVG. chart.SVG writes numbers, constant
// attributes and escaped text only, so its output is trusted as HTML.
func drawChart(w deliverable.Widget) (template.HTML, error) {
	d, err := chart.Layout(w)
	if err != nil {
		return "", err
	}
	return template.HTML(chart.SVG(d)), nil
}

// columns is a section's grid width: its own, or the kind's default (a
// dashboard two, a report one).
func columns(kind string, sec deliverable.Section) int {
	switch {
	case sec.Columns > 0:
		return sec.Columns
	case kind == deliverable.Dashboard:
		return 2
	}
	return 1
}

// decimals is a widget's decimals, or the format's own when none is given.
func decimals(d *int) int {
	if d == nil {
		return -1
	}
	return *d
}

// metric is a metric's value in its format, or a dash while a bound value is
// not yet read.
func metric(w deliverable.Widget) string {
	if w.Value == nil {
		return "—"
	}
	return deliverable.Format(*w.Value, w.Format, decimals(w.Decimals))
}

// delta is a metric's change with its sign, in the metric's format.
func delta(w deliverable.Widget) string {
	if w.Delta == nil {
		return ""
	}
	s := deliverable.Format(w.Delta.Value, w.Format, decimals(w.Decimals))
	if w.Delta.Value > 0 {
		return "+" + s
	}
	return s
}

// deltaTone is positive for a good change, negative for a bad one and
// neutral when the spec does not say. The sign is always written beside it.
// See docs/design.md#colour.
func deltaTone(d *deliverable.Delta) string {
	switch {
	case d == nil || d.Good == nil:
		return "neutral"
	case *d.Good:
		return "positive"
	}
	return "negative"
}

// progress is "done of total" in words.
func progress(w deliverable.Widget) string {
	if w.Done == nil || w.Total == nil {
		return "—"
	}
	return deliverable.Format(*w.Done, "number", -1) + " of " + deliverable.Format(*w.Total, "number", -1)
}

// percent is the filled share of a progress bar, 0 to 100, as the width of a
// rectangle in a 100-unit box.
func percent(w deliverable.Widget) string {
	if w.Done == nil || w.Total == nil || *w.Total <= 0 {
		return "0"
	}
	p := min(max(*w.Done / *w.Total * 100, 0), 100)
	return deliverable.Format(p, "number", -1)
}

// cell is one table cell as written: its text and its column's alignment.
type cell struct {
	Text  string
	Align string
}

// cells is a row of a table as text, a number in its column's format. A cell
// past the last column is left aligned in the number format.
func cells(w deliverable.Widget, row []any) []cell {
	out := make([]cell, len(row))
	for i, v := range row {
		var col deliverable.Column
		if i < len(w.Columns) {
			col = w.Columns[i]
		}
		switch v := v.(type) {
		case string:
			out[i].Text = v
		case float64:
			out[i].Text = deliverable.Format(v, col.Format, -1)
		}
		out[i].Align = col.Align
	}
	return out
}

// asOf writes the time a bound value was read (RFC 3339, UTC, to the minute)
// as a date and a time: "2026-10-20 09:30 UTC".
func asOf(s string) string {
	if t, ok := strings.CutSuffix(s, "Z"); ok {
		return strings.Replace(t, "T", " ", 1) + " UTC"
	}
	return s
}
