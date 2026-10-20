package widget

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Validate refuses a Spec the catalogue does not allow. Each refusal says what
// to change.
func Validate(s Spec) error {
	e, ok := lookup(s.Kind)
	if !ok {
		names := make([]string, 0, len(catalogue))
		for _, k := range Kinds() {
			names = append(names, string(k))
		}
		return refuse("the kind %q is not one of the controls; use one of: %s", s.Kind, strings.Join(names, ", "))
	}
	if got := s.fields(); len(got) != 1 || got[0] != s.Kind {
		return refuse("%s carries the settings of exactly one control, its own; you gave %s", e.noun, listKinds(got))
	}
	switch s.Kind {
	case SingleChoice:
		return validateOptions(e, s.Single.Options)
	case MultipleChoice:
		return validateMultiple(e, s.Multiple)
	case FreeText:
		return validateFreeText(s.FreeText)
	case YesNo:
		return validateLabels([2]string{"yes_label", s.YesNo.YesLabel}, [2]string{"no_label", s.YesNo.NoLabel})
	case Date:
		return validateDate(s.Date)
	case Path:
		if !s.Path.Folders && !s.Path.Files {
			return refuse("a file or folder pick needs folders, files or both set to true")
		}
		return nil
	default:
		return validateScale(s.Scale)
	}
}

// fields lists the kinds whose settings are present, in catalogue order.
func (s Spec) fields() []Kind {
	set := []bool{s.Single != nil, s.Multiple != nil, s.FreeText != nil, s.YesNo != nil, s.Date != nil, s.Path != nil, s.Scale != nil}
	var got []Kind
	for i, e := range catalogue {
		if set[i] {
			got = append(got, e.kind)
		}
	}
	return got
}

func listKinds(ks []Kind) string {
	if len(ks) == 0 {
		return "none"
	}
	names := make([]string, len(ks))
	for i, k := range ks {
		names[i] = string(k)
	}
	return strings.Join(names, " and ")
}

func refuse(format string, args ...any) error {
	return invalid(fmt.Sprintf(format, args...))
}

func validateOptions(e entry, opts []Option) error {
	lo, hi := optionBounds[e.kind][0], optionBounds[e.kind][1]
	if len(opts) < lo || len(opts) > hi {
		return refuse("%s needs %d to %d options; you gave %d", e.noun, lo, hi, len(opts))
	}
	seen := make(map[string]int, len(opts))
	for i, o := range opts {
		n := i + 1
		if strings.TrimSpace(o.ID) == "" {
			return refuse("option %d has an empty id; give every option a short id such as \"a\"", n)
		}
		if first, dup := seen[o.ID]; dup {
			return refuse("options %d and %d share the id %q; ids must be unique", first, n, o.ID)
		}
		seen[o.ID] = n
		if strings.TrimSpace(o.Label) == "" {
			return refuse("option %d has an empty label; say what the owner reads", n)
		}
		if l := utf8.RuneCountInString(o.Label); l > MaxLabel {
			return refuse("the label of option %d is %d characters; keep labels to %d or fewer", n, l, MaxLabel)
		}
		if err := Plain(fmt.Sprintf("the label of option %d", n), o.Label); err != nil {
			return err
		}
		if err := Plain(fmt.Sprintf("the detail of option %d", n), o.Detail); err != nil {
			return err
		}
	}
	return nil
}

// Plain refuses s, named what in the refusal, when it holds a control
// character such as a line break or a tab. Words the owner picks become
// their message in the thread, so a second line hidden in a control would
// read there as something they said.
// See docs/model.md#owner-interaction.
func Plain(what, s string) error {
	if strings.ContainsFunc(s, unicode.IsControl) {
		return refuse("%s holds a line break or another control character; write it on one line as plain text", what)
	}
	return nil
}

func validateMultiple(e entry, m *MultipleSpec) error {
	if err := validateOptions(e, m.Options); err != nil {
		return err
	}
	if m.MinChoices < 0 || m.MaxChoices < 0 {
		return refuse("min_choices and max_choices cannot be negative")
	}
	if m.MaxChoices > len(m.Options) {
		return refuse("max_choices is %d but there are only %d options", m.MaxChoices, len(m.Options))
	}
	if m.MaxChoices > 0 && m.MinChoices > m.MaxChoices {
		return refuse("min_choices %d is more than max_choices %d", m.MinChoices, m.MaxChoices)
	}
	if m.MaxChoices == 0 && m.MinChoices > len(m.Options) {
		return refuse("min_choices is %d but there are only %d options", m.MinChoices, len(m.Options))
	}
	return nil
}

func validateFreeText(f *FreeTextSpec) error {
	if f.MaxLength < 0 || f.MaxLength > MaxText {
		return refuse("free text takes a max_length of 1 to %d; you gave %d", MaxText, f.MaxLength)
	}
	if l := utf8.RuneCountInString(f.Placeholder); l > MaxPlaceholder {
		return refuse("the placeholder is %d characters; keep it to %d or fewer", l, MaxPlaceholder)
	}
	return Plain("the placeholder", f.Placeholder)
}

func validateLabels(pairs ...[2]string) error {
	for _, p := range pairs {
		if n := utf8.RuneCountInString(p[1]); n > MaxLabel {
			return refuse("%s is %d characters; keep it to %d or fewer", p[0], n, MaxLabel)
		}
		if err := Plain(p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}

func validateDate(d *DateSpec) error {
	var from, to time.Time
	var err error
	if d.Earliest != "" {
		if from, err = time.Parse(DateLayout, d.Earliest); err != nil {
			return refuse("earliest %q is not a day written YYYY-MM-DD", d.Earliest)
		}
	}
	if d.Latest != "" {
		if to, err = time.Parse(DateLayout, d.Latest); err != nil {
			return refuse("latest %q is not a day written YYYY-MM-DD", d.Latest)
		}
	}
	if d.Earliest != "" && d.Latest != "" && to.Before(from) {
		return refuse("earliest %s is after latest %s; put the earlier day first", d.Earliest, d.Latest)
	}
	return nil
}

func validateScale(c *ScaleSpec) error {
	if c.Min >= c.Max {
		return refuse("a scale needs min below max; you gave min %d and max %d", c.Min, c.Max)
	}
	if steps := c.Max - c.Min + 1; steps > MaxScaleSteps {
		return refuse("a scale takes at most %d steps; %d to %d is %d", MaxScaleSteps, c.Min, c.Max, steps)
	}
	return validateLabels([2]string{"min_label", c.MinLabel}, [2]string{"max_label", c.MaxLabel})
}
