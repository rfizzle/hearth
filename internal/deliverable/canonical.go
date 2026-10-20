package deliverable

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

// Canonical writes the one form of a Spec: sorted keys, two-space indent, a
// newline at the end, defaults left out and the kind's default columns written
// out, and numbers in encoding/json's one shortest form. Two inputs that mean
// the same spec give the same bytes, and Canonical of a Parse of Canonical is
// Canonical.
func Canonical(s Spec) ([]byte, error) {
	flat, err := marshal(normalise(s))
	if err != nil {
		return nil, refuse("the spec cannot be written: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(flat))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, refuse("the spec cannot be written: %v", err)
	}
	return marshal(unsign(tree)) // maps write their keys sorted
}

func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return b.Bytes(), err
}

// unsign rewrites a negative zero as zero, the one number with two spellings.
func unsign(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = unsign(x)
		}
	case []any:
		for i, x := range t {
			t[i] = unsign(x)
		}
	case json.Number:
		if f, err := t.Float64(); err == nil && f == 0 {
			return json.Number("0")
		}
	}
	return v
}

// normalise is a copy of s with every default written the one way: columns
// explicit, and a span of one, a format of number, a style of bullet and an
// alignment of left left out.
func normalise(s Spec) Spec {
	s.Sections = slices.Clone(s.Sections)
	for i, sec := range s.Sections {
		sec.Columns = columnsOf(s.Kind, sec)
		sec.Widgets = slices.Clone(sec.Widgets)
		for j, w := range sec.Widgets {
			w.Span = ifNot(w.Span, 1)
			w.Format = ifNot(w.Format, "number")
			w.Style = ifNot(w.Style, "bullet")
			w.Columns = slices.Clone(w.Columns)
			for k, c := range w.Columns {
				w.Columns[k] = Column{Header: c.Header, Align: ifNot(c.Align, "left"), Format: ifNot(c.Format, "number")}
			}
			sec.Widgets[j] = w
		}
		s.Sections[i] = sec
	}
	return s
}

// ifNot is the zero value of v's type when v is the default, else v.
func ifNot[T comparable](v, def T) T {
	if v == def {
		var zero T
		return zero
	}
	return v
}

// Format writes v the one way every renderer does, with no locale: thousands
// separated by commas, a point before the decimals. A currency is $ with two
// decimals, a percent has one and a % after it, an integer rounds half away
// from zero, and a number shows the decimals asked for or else up to three
// without trailing zeros. A decimals below zero asks for the format's own.
func Format(v float64, format string, decimals int) string {
	d, trim := decimals, false
	switch {
	case format == "integer":
		d = 0
	case decimals >= 0:
	case format == "currency":
		d = 2
	case format == "percent":
		d = 1
	default:
		d, trim = 3, true
	}
	scale := math.Pow10(d)
	r := math.Round(v*scale) / scale
	digits := strconv.FormatFloat(math.Abs(r), 'f', d, 64)
	if trim && strings.Contains(digits, ".") {
		digits = strings.TrimRight(strings.TrimRight(digits, "0"), ".")
	}
	whole, frac, _ := strings.Cut(digits, ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	if frac != "" {
		whole += "." + frac
	}
	switch format {
	case "currency":
		whole = "$" + whole
	case "percent":
		whole += "%"
	}
	if r < 0 {
		return "-" + whole
	}
	return whole
}
