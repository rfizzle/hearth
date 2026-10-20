package deliverable

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"sort"
	"strings"
)

// maxDepth is how deep the JSON may nest. The schema itself reaches eight.
const maxDepth = 10

// Parse reads a Spec a model wrote. Text that is not JSON, a null, a field the
// catalogue does not know, a number given as text, and a spec over MaxBytes are
// refusals that name where they are. The result is not yet validated.
func Parse(data []byte) (Spec, error) {
	if len(data) > MaxBytes {
		return Spec{}, refuse("the deliverable is %d KB; the limit is %d KB", (len(data)+1023)/1024, MaxBytes>>10)
	}
	if err := scan(data); err != nil {
		return Spec{}, err
	}
	var in struct {
		Version  int               `json:"version"`
		Kind     string            `json:"kind"`
		Title    string            `json:"title"`
		Subtitle string            `json:"subtitle"`
		Template string            `json:"template"`
		Sections []json.RawMessage `json:"sections"`
	}
	if err := strict(data, &in); err != nil {
		return Spec{}, decodeErr(err, "deliverable")
	}
	s := Spec{Version: in.Version, Kind: in.Kind, Title: in.Title, Subtitle: in.Subtitle, Template: in.Template}
	for i, raw := range in.Sections {
		sec, err := parseSection(raw, i+1)
		if err != nil {
			return Spec{}, err
		}
		s.Sections = append(s.Sections, sec)
	}
	return s, nil
}

func parseSection(raw json.RawMessage, n int) (Section, error) {
	var in struct {
		Heading string            `json:"heading"`
		Columns int               `json:"columns"`
		Widgets []json.RawMessage `json:"widgets"`
	}
	if err := strict(raw, &in); err != nil {
		return Section{}, prefix(fmt.Sprintf("section %d", n), decodeErr(err, "section"))
	}
	sec := Section{Heading: in.Heading, Columns: in.Columns}
	for j, wr := range in.Widgets {
		var w Widget
		if err := w.UnmarshalJSON(wr); err != nil {
			return Section{}, prefix(fmt.Sprintf("section %d, widget %d", n, j+1), err)
		}
		sec.Widgets = append(sec.Widgets, w)
	}
	return sec, nil
}

// UnmarshalJSON reads one widget strictly: a field the widget's kind does not
// take is a refusal, as is a number written as text. An unknown kind leaves
// every field of the kind unread for Validate to refuse.
func (w *Widget) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return decodeErr(err, "widget")
	}
	var kind Kind
	if raw, ok := keys["kind"]; ok {
		if err := json.Unmarshal(raw, &kind); err != nil {
			return decodeErr(err, "widget")
		}
	}
	noun := string(kind)
	if i := slices.IndexFunc(catalogue, func(e entry) bool { return e.kind == kind }); i >= 0 {
		names := make([]string, 0, len(keys))
		for k := range keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			if !takes(catalogue[i], k) {
				return unknownField(noun, k)
			}
		}
	} else {
		noun = "widget"
	}
	type bare Widget // no methods, so no recursion
	var b bare
	if err := strict(data, &b); err != nil {
		return decodeErr(err, noun)
	}
	*w = Widget(b)
	return nil
}

func takes(e entry, field string) bool {
	switch field {
	case "kind", "span", "as_of":
		return true
	}
	return slices.Contains(strings.Fields(e.fields), field)
}

func unknownField(noun, field string) error {
	if field == "measure" {
		return refuse("a %s cannot bind a measure; only metric, progress and light take one", noun)
	}
	return refuse("the field %q is not one a %s takes", field, noun)
}

// strict decodes data into v and refuses fields v does not have.
func strict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// scan refuses text that is not one JSON value, a null anywhere, and nesting
// deeper than the schema ever reaches.
func scan(data []byte) error {
	if !json.Valid(data) {
		var x any
		return refuse("the spec is not valid JSON: %v", json.Unmarshal(data, &x))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // a number out of range is the decoder's to refuse, in words
	depth := 0
	for {
		t, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return refuse("the spec is not valid JSON: %v", err)
		}
		switch t := t.(type) {
		case nil:
			return refuse("the spec has a null; leave a field out rather than writing null")
		case json.Delim:
			depth += delta(t)
			if depth > maxDepth {
				return refuse("the spec nests more than %d levels deep; flatten it", maxDepth)
			}
		}
	}
}

// decodeErr turns a decoder error into a refusal in words; noun is the thing
// being read ("section", "metric").
func decodeErr(err error, noun string) error {
	if errors.Is(err, ErrInvalid) {
		return err
	}
	if f, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
		return unknownField(noun, strings.Trim(f, `"`))
	}
	var te *json.UnmarshalTypeError
	if !errors.As(err, &te) {
		return refuse("the %s is not valid JSON: %v", noun, err)
	}
	switch {
	case strings.HasPrefix(te.Value, "number ") && te.Type.Kind() != reflect.String:
		return refuse("the value %s is too large; numbers go up to 15 digits", strings.TrimPrefix(te.Value, "number "))
	case te.Value == "string" && slices.Contains([]reflect.Kind{reflect.Float64, reflect.Int}, te.Type.Kind()):
		return refuse("values are numbers, not text (the field %q)", te.Field)
	}
	if te.Field == "" {
		return refuse("the %s wants %s, not %s", noun, wants(te.Type.Kind()), te.Value)
	}
	return refuse("the field %q wants %s, not %s", te.Field, wants(te.Type.Kind()), te.Value)
}

func wants(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "text"
	case reflect.Float64, reflect.Int:
		return "a number"
	case reflect.Bool:
		return "true or false"
	case reflect.Slice:
		return "a list"
	}
	return "an object"
}

func refuse(format string, args ...any) error {
	return invalid(fmt.Sprintf(format, args...))
}

// prefix puts where in front of a refusal.
func prefix(where string, err error) error {
	return invalid(where + ": " + err.Error())
}

// delta is one level deeper for an opening bracket and one up for a closing.
func delta(d json.Delim) int {
	if d == '{' || d == '[' {
		return 1
	}
	return -1
}
