package widget

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// specWire is the flat JSON of a Spec, with kind as the discriminator. The
// settings of each kind are embedded so their fields sit beside kind.
type specWire struct {
	Kind     Kind     `json:"kind"`
	Required bool     `json:"required,omitempty"`
	Options  []Option `json:"options,omitempty"`
	SingleSpec
	MultipleSpec
	FreeTextSpec
	YesNoSpec
	DateSpec
	PathSpec
	ScaleSpec
}

// val is the value p points at, or the zero value.
func val[T any](p *T) T {
	var z T
	if p != nil {
		z = *p
	}
	return z
}

// MarshalJSON writes the flat form.
func (s Spec) MarshalJSON() ([]byte, error) {
	w := specWire{Kind: s.Kind, Required: s.Required}
	w.SingleSpec, w.MultipleSpec, w.FreeTextSpec = val(s.Single), val(s.Multiple), val(s.FreeText)
	w.YesNoSpec, w.DateSpec, w.PathSpec, w.ScaleSpec = val(s.YesNo), val(s.Date), val(s.Path), val(s.Scale)
	w.Options = append(w.SingleSpec.Options, w.MultipleSpec.Options...)
	return json.Marshal(w)
}

// UnmarshalJSON reads the flat form and sets the field its kind names. An
// unknown kind leaves every field nil, which Validate refuses.
func (s *Spec) UnmarshalJSON(data []byte) error {
	var w specWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*s = Spec{Kind: w.Kind, Required: w.Required}
	switch w.Kind {
	case SingleChoice:
		s.Single = &w.SingleSpec
		s.Single.Options = w.Options
	case MultipleChoice:
		s.Multiple = &w.MultipleSpec
		s.Multiple.Options = w.Options
	case FreeText:
		s.FreeText = &w.FreeTextSpec
	case YesNo:
		s.YesNo = &w.YesNoSpec
	case Date:
		s.Date = &w.DateSpec
	case Path:
		s.Path = &w.PathSpec
	case Scale:
		s.Scale = &w.ScaleSpec
	}
	return nil
}

// decodeStrict decodes data into v and refuses fields v does not know.
func decodeStrict(data []byte, what string, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return invalid(fmt.Sprintf("the %s is not valid: %v", what, err))
	}
	return nil
}

// ParseSpec reads a Spec a model wrote. Malformed JSON and fields the
// catalogue does not know are refusals; the result is not yet validated.
func ParseSpec(data []byte) (Spec, error) {
	var w specWire
	if err := decodeStrict(data, "widget", &w); err != nil {
		return Spec{}, err
	}
	var s Spec
	return s, s.UnmarshalJSON(data)
}

// ParseAnswer reads an Answer from its stored JSON.
func ParseAnswer(data []byte) (Answer, error) {
	var a Answer
	err := decodeStrict(data, "answer", &a)
	return a, err
}
