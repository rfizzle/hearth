package deliverable

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The limits of a spec, in the words the refusals use.
const (
	maxNumber  = 1e15
	maxTotal   = 1e9
	maxWidgets = 40
	asOfLayout = "2006-01-02T15:04Z"
)

func formats() []string { return []string{"number", "integer", "currency", "percent"} }
func tones() []string   { return []string{"neutral", "positive", "attention", "negative"} }

// Validate refuses a Spec the catalogue does not allow. It returns the first
// refusal, in words, wrapping ErrInvalid, naming the section and the widget.
func Validate(s Spec) error {
	if s.Version != 1 {
		return refuse("this Hearth writes version 1 deliverables; set version to 1")
	}
	if s.Kind != Report && s.Kind != Dashboard {
		return refuse("kind is report or dashboard; you gave %q", s.Kind)
	}
	if err := firstErr(text("the title", s.Title, 1, 120), text("the subtitle", s.Subtitle, 0, 160),
		bounds("the spec", "sections", len(s.Sections), 1, 12)); err != nil {
		return err
	}
	total := 0
	for i, sec := range s.Sections {
		total += len(sec.Widgets)
		if err := validateSection(sec); err != nil {
			return prefix(fmt.Sprintf("section %d", i+1), err)
		}
		for j, w := range sec.Widgets {
			if err := validateWidget(w, columnsOf(s.Kind, sec)); err != nil {
				return prefix(fmt.Sprintf("section %d, widget %d", i+1, j+1), err)
			}
		}
	}
	return bounds("the spec", "widgets", total, 0, maxWidgets)
}

// Authored refuses what only Hearth may set: the template and the time a bound
// value was read. The tool calls it on what a bot sent, before Validate.
func Authored(s Spec) error {
	if s.Template != "" {
		return refuse("the template is set from the tool's template argument; leave it out of the spec")
	}
	for i, sec := range s.Sections {
		for j, w := range sec.Widgets {
			if w.AsOf != "" {
				return refuse("section %d, widget %d: as_of is filled in when Hearth writes a bound value; leave it out", i+1, j+1)
			}
		}
	}
	return nil
}

// columnsOf is a section's columns with the kind's default filled in.
func columnsOf(kind string, sec Section) int {
	switch {
	case sec.Columns > 0:
		return sec.Columns
	case kind == Dashboard:
		return 2
	}
	return 1
}

func validateSection(sec Section) error {
	if sec.Columns < 0 || sec.Columns > 3 {
		return refuse("columns go from 1 to 3; you gave %d", sec.Columns)
	}
	return firstErr(text("the heading", sec.Heading, 0, 80), bounds("the section", "widgets", len(sec.Widgets), 1, 12))
}

func validateWidget(w Widget, cols int) error {
	if w.Span < 0 || w.Span > cols {
		return refuse("the widget spans %d columns; the section has %d", w.Span, cols)
	}
	if !slices.ContainsFunc(catalogue, func(e entry) bool { return e.kind == w.Kind }) {
		return refuse("the kind %q is not one of the widgets; use one of: %s", w.Kind, joinKinds())
	}
	if err := bindings(w); err != nil {
		return err
	}
	switch w.Kind {
	case Metric:
		return validateMetric(w)
	case Line, Bar:
		return validateChart(w)
	case Pie:
		return validatePie(w)
	case Table:
		return validateTable(w)
	case List:
		return validateList(w)
	case Progress:
		return validateProgress(w)
	case Light:
		return validateLight(w)
	case Callout:
		return firstErr(oneOf("tone", w.Tone, true, tones()...), text("the title", w.Title, 0, 80), markdown("callout", w.Markdown, 2000))
	}
	return markdown("text", w.Markdown, 20000)
}

func joinKinds() string {
	names := make([]string, len(catalogue))
	for i, e := range catalogue {
		names[i] = string(e.kind)
	}
	return strings.Join(names, ", ")
}

// bindings checks the measure id and the time it was read.
func bindings(w Widget) error {
	if w.Measure == "" && w.AsOf == "" {
		return nil
	}
	if w.Measure == "" {
		return refuse("as_of belongs to a bound widget, and this one has no measure")
	}
	if w.Kind != Metric && w.Kind != Progress && w.Kind != Light {
		return refuse("a %s cannot bind a measure; only metric, progress and light take one", w.Kind)
	}
	if err := text("the measure", w.Measure, 1, 80); err != nil {
		return err
	}
	if i := strings.IndexFunc(w.Measure, func(r rune) bool { return !measureRune(r) }); i >= 0 {
		return refuse("the measure %q holds a character an id does not use; copy it from the list of measures", w.Measure)
	}
	if _, err := time.Parse(asOfLayout, w.AsOf); w.AsOf != "" && err != nil {
		return refuse("as_of is a time in UTC to the minute, like 2026-10-20T09:30Z; you gave %q", w.AsOf)
	}
	return nil
}

func measureRune(r rune) bool {
	return r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._/-:", r))
}

func validateMetric(w Widget) error {
	if w.Value == nil && w.Measure == "" {
		return refuse("a metric needs a value or a measure")
	}
	if w.Decimals != nil && (*w.Decimals < 0 || *w.Decimals > 3) {
		return refuse("decimals go from 0 to 3; you gave %d", *w.Decimals)
	}
	errs := []error{text("the label", w.Label, 1, 40), text("the unit", w.Unit, 0, 12), text("the caption", w.Caption, 0, 80),
		oneOf("format", w.Format, false, formats()...), optNumber("the value", w.Value)}
	if w.Delta != nil {
		errs = append(errs, number("the delta", w.Delta.Value))
	}
	return firstErr(errs...)
}

// titled checks what a chart and a pie share: a title and a number format.
func titled(w Widget) []error {
	return []error{text("the title", w.Title, 1, 80), oneOf("format", w.Format, false, formats()...)}
}

func validateChart(w Widget) error {
	errs := append(titled(w), text("the y label", w.YLabel, 0, 24),
		bounds("the chart", "categories", len(w.Categories), 1, 52), bounds("the chart", "series", len(w.Series), 1, 6),
		optNumber("y_min", w.YMin), optNumber("y_max", w.YMax))
	if w.YMin != nil && w.YMax != nil && *w.YMin >= *w.YMax {
		errs = append(errs, refuse("y_min %s is not below y_max %s", ftoa(*w.YMin), ftoa(*w.YMax)))
	}
	for i, c := range w.Categories {
		errs = append(errs, text(fmt.Sprintf("category %d", i+1), c, 1, 24))
	}
	for _, s := range w.Series {
		errs = append(errs, text("a series name", s.Name, 1, 40), checkSeries(s, len(w.Categories)))
	}
	return firstErr(errs...)
}

func checkSeries(s Series, categories int) error {
	if len(s.Values) != categories {
		return refuse("series %q has %d values for %d categories; give one value per category", s.Name, len(s.Values), categories)
	}
	for _, v := range s.Values {
		if err := number("a value of series "+strconv.Quote(s.Name), v); err != nil {
			return err
		}
	}
	return nil
}

func validatePie(w Widget) error {
	errs := append(titled(w), bounds("the pie", "slices", len(w.Slices), 2, 8))
	sum := 0.0
	for _, s := range w.Slices {
		errs = append(errs, text("a slice label", s.Label, 1, 40), number("the slice "+strconv.Quote(s.Label), s.Value))
		if s.Value < 0 {
			errs = append(errs, refuse("slice %q is negative; a pie takes zero or more", s.Label))
		}
		sum += s.Value
	}
	if sum == 0 {
		errs = append(errs, refuse("every slice is zero; a pie needs something to show"))
	}
	return firstErr(errs...)
}

func validateTable(w Widget) error {
	errs := []error{text("the title", w.Title, 0, 80), bounds("the table", "columns", len(w.Columns), 1, 8),
		bounds("the table", "rows", len(w.Rows), 1, 100)}
	for _, c := range w.Columns {
		errs = append(errs, text("a column header", c.Header, 1, 40), oneOf("align", c.Align, false, "left", "center", "right"),
			oneOf("format", c.Format, false, formats()...))
	}
	for i, row := range w.Rows {
		errs = append(errs, checkRow(fmt.Sprintf("row %d", i+1), row, len(w.Columns)))
	}
	if w.Footer != nil {
		errs = append(errs, checkRow("the footer", w.Footer, len(w.Columns)))
	}
	return firstErr(errs...)
}

func checkRow(name string, row []any, columns int) error {
	if len(row) != columns {
		return refuse("%s has %d cells for %d columns; give one cell per column", name, len(row), columns)
	}
	for i, c := range row {
		what := fmt.Sprintf("cell %d of %s", i+1, name)
		switch v := c.(type) {
		case string:
			if err := text(what, v, 0, 120); err != nil {
				return err
			}
		case float64:
			if err := number(what, v); err != nil {
				return err
			}
		default:
			return refuse("%s is text or a number", what)
		}
	}
	return nil
}

func validateList(w Widget) error {
	errs := []error{text("the title", w.Title, 0, 80), oneOf("style", w.Style, false, "bullet", "numbered", "check"),
		bounds("the list", "items", len(w.Items), 1, 50)}
	for i, it := range w.Items {
		errs = append(errs, text(fmt.Sprintf("item %d", i+1), it.Text, 1, 200), text(fmt.Sprintf("the detail of item %d", i+1), it.Detail, 0, 120))
		if it.Done && w.Style != "check" {
			errs = append(errs, refuse("item %d has done but the list is not a check list; set style to check", i+1))
		}
	}
	return firstErr(errs...)
}

func validateProgress(w Widget) error {
	errs := []error{text("the label", w.Label, 1, 40), text("the caption", w.Caption, 0, 80), optNumber("done", w.Done), optNumber("total", w.Total)}
	switch {
	case w.Done == nil && w.Total == nil && w.Measure == "":
		errs = append(errs, refuse("a progress widget needs done and total, or a measure"))
	case (w.Done == nil) != (w.Total == nil):
		errs = append(errs, refuse("give both done and total, or neither"))
	case w.Done != nil && *w.Done < 0:
		errs = append(errs, refuse("done %s is below zero", ftoa(*w.Done)))
	case w.Done != nil && *w.Total > maxTotal:
		errs = append(errs, refuse("total %s is too large; the limit is 1000000000", ftoa(*w.Total)))
	case w.Done != nil && *w.Done > *w.Total:
		errs = append(errs, refuse("done %s is more than total %s", ftoa(*w.Done), ftoa(*w.Total)))
	}
	return firstErr(errs...)
}

func validateLight(w Widget) error {
	errs := []error{text("the label", w.Label, 1, 40), text("the caption", w.Caption, 0, 80), oneOf("tone", w.Tone, false, tones()...)}
	switch {
	case w.Tone == "" && w.Word == "" && w.Measure == "":
		errs = append(errs, refuse("a light needs a tone and a word, or a measure"))
	case (w.Tone == "") != (w.Word == ""):
		errs = append(errs, refuse("give tone and word together; the word always travels with the tone"))
	case w.Word != "":
		errs = append(errs, text("the word", w.Word, 1, 24))
	}
	return firstErr(errs...)
}

// firstErr is the first refusal among checks that were all run.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// plain refuses a control character; multiline text may hold line breaks and
// tabs.
func plain(what, s string, multiline bool) error {
	bad := func(r rune) bool { return unicode.IsControl(r) && (!multiline || (r != '\n' && r != '\t')) }
	if !utf8.ValidString(s) || strings.ContainsFunc(s, bad) {
		if multiline {
			return refuse("%s holds a control character other than line breaks and tabs; remove it", what)
		}
		return refuse("%s holds a line break or another control character; write it on one line", what)
	}
	return nil
}

// text refuses plain text outside min to max characters.
func text(what, s string, lo, hi int) error {
	if err := plain(what, s, false); err != nil {
		return err
	}
	return count(what, utf8.RuneCountInString(s), lo, hi, "characters")
}

// count refuses n outside lo to hi, naming the unit.
func count(what string, n, lo, hi int, unit string) error {
	switch {
	case n > hi:
		return refuse("%s has %d %s; the limit is %d", what, n, unit, hi)
	case n == 0 && lo > 0:
		return refuse("%s is empty; give it %d to %d %s", what, lo, hi, unit)
	case n < lo:
		return refuse("%s has %d %s; give it %d to %d", what, n, unit, lo, hi)
	}
	return nil
}

func bounds(subject, noun string, n, lo, hi int) error {
	if n > hi {
		return refuse("%s has %d %s; the limit is %d", subject, n, noun, hi)
	}
	if n < lo {
		return refuse("%s has %d %s; give %d to %d", subject, n, noun, lo, hi)
	}
	return nil
}

// number refuses a value that is not finite or is beyond fifteen digits.
func number(what string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return refuse("%s is not a finite number; write plain digits", what)
	}
	if math.Abs(v) > maxNumber {
		return refuse("the value %s is too large; numbers go up to 15 digits", strconv.FormatFloat(v, 'g', -1, 64))
	}
	return nil
}

func optNumber(what string, v *float64) error {
	if v == nil {
		return nil
	}
	return number(what, *v)
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// oneOf refuses a value outside the allowed words; an empty value is the
// default unless required.
func oneOf(what, s string, required bool, allowed ...string) error {
	if (s == "" && !required) || slices.Contains(allowed, s) {
		return nil
	}
	list := strings.Join(allowed[:len(allowed)-1], ", ") + " or " + allowed[len(allowed)-1]
	return refuse("%s is %s; you gave %q", what, list, s)
}
