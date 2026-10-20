package widget

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// Check refuses an Answer the Spec does not allow. The Spec is taken to have
// passed Validate. An empty Answer to a control that is not required is
// allowed: the owner declined it.
func Check(s Spec, a Answer) error {
	if a.Kind != s.Kind {
		return refuse("this question asks for %s but the answer is for %q", nounOf(s.Kind), a.Kind)
	}
	if a.empty() {
		if s.Required || (s.Multiple != nil && s.Multiple.MinChoices > 0) {
			return refuse("%s needs an answer and the control was left empty", nounOf(s.Kind))
		}
		return nil
	}
	switch s.Kind {
	case SingleChoice:
		return checkSingle(s.Single, a)
	case MultipleChoice:
		return checkMultiple(s.Multiple, a)
	case FreeText:
		return checkText(s.FreeText, a)
	case YesNo:
		return nil
	case Date:
		return checkDate(s.Date, a)
	case Path:
		return checkPath(s.Path, a)
	case Scale:
		return checkScale(s.Scale, a)
	}
	return refuse("the kind %q is not one of the controls", s.Kind)
}

func nounOf(k Kind) string {
	if e, ok := lookup(k); ok {
		return e.noun
	}
	return "an unknown control"
}

// checkIDs refuses ids the options do not have, and repeats.
func checkIDs(opts []Option, ids []string) error {
	for i, id := range ids {
		if !slices.ContainsFunc(opts, func(o Option) bool { return o.ID == id }) {
			return refuse("%q is not one of the option ids", id)
		}
		if slices.Contains(ids[:i], id) {
			return refuse("%q was chosen twice", id)
		}
	}
	return nil
}

func checkOther(allowed bool, other string) error {
	if other == "" {
		return nil
	}
	if !allowed {
		return refuse("this question does not take an Other answer")
	}
	if n := utf8.RuneCountInString(other); n > MaxOther {
		return refuse("the Other text is %d characters; the limit is %d", n, MaxOther)
	}
	return nil
}

func checkSingle(sp *SingleSpec, a Answer) error {
	if len(a.Choices) > 1 {
		return refuse("a single choice takes one option; the answer has %d", len(a.Choices))
	}
	if len(a.Choices) == 1 && a.Other != "" {
		return refuse("a single choice takes one option or Other, not both")
	}
	if err := checkIDs(sp.Options, a.Choices); err != nil {
		return err
	}
	return checkOther(sp.AllowOther, a.Other)
}

func checkMultiple(m *MultipleSpec, a Answer) error {
	if err := checkIDs(m.Options, a.Choices); err != nil {
		return err
	}
	if err := checkOther(false, a.Other); err != nil {
		return err
	}
	n := len(a.Choices)
	if a.Other != "" {
		n++
	}
	if n < m.MinChoices {
		return refuse("a multiple choice needs at least %d choices; the answer has %d", m.MinChoices, n)
	}
	if m.MaxChoices > 0 && n > m.MaxChoices {
		return refuse("a multiple choice takes at most %d choices; the answer has %d", m.MaxChoices, n)
	}
	return nil
}

func checkText(f *FreeTextSpec, a Answer) error {
	limit := f.MaxLength
	if limit == 0 {
		limit = DefaultText
	}
	if n := utf8.RuneCountInString(a.Text); n > limit {
		return refuse("the text is %d characters; the limit is %d", n, limit)
	}
	return nil
}

func checkDate(d *DateSpec, a Answer) error {
	layout, form := DateLayout, "YYYY-MM-DD"
	if d.WithTime {
		layout, form = DateTimeLayout, "YYYY-MM-DD HH:MM"
	}
	at, err := time.Parse(layout, a.Date)
	if err != nil {
		return refuse("the date %q is not written %s", a.Date, form)
	}
	day := at.Truncate(24 * time.Hour)
	if d.Earliest != "" {
		if from, err := time.Parse(DateLayout, d.Earliest); err == nil && day.Before(from) {
			return refuse("the date %s is before the earliest allowed, %s", a.Date, d.Earliest)
		}
	}
	if d.Latest != "" {
		if to, err := time.Parse(DateLayout, d.Latest); err == nil && day.After(to) {
			return refuse("the date %s is after the latest allowed, %s", a.Date, d.Latest)
		}
	}
	return nil
}

func checkPath(_ *PathSpec, a Answer) error {
	if strings.TrimSpace(a.Path) == "" {
		return refuse("the path is empty")
	}
	if len(a.Path) > MaxPath || strings.ContainsRune(a.Path, 0) {
		return refuse("the path is not a usable path (at most %d bytes, no NUL)", MaxPath)
	}
	return nil
}

func checkScale(c *ScaleSpec, a Answer) error {
	if a.Value == nil {
		return refuse("a scale answer needs a value from %d to %d", c.Min, c.Max)
	}
	if *a.Value < c.Min || *a.Value > c.Max {
		return refuse("the value %d is outside the scale %d to %d", *a.Value, c.Min, c.Max)
	}
	return nil
}
