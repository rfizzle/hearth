package project

import (
	"errors"
	"testing"
)

// Owner-typed text and a bot's proposal reach later prompts through the
// stored brief, so an invisible character is refused in the name and stripped
// from the parts.
func TestName_RefusesInvisibleCharacters(t *testing.T) {
	for _, name := range []string{
		"Launch\u202e", "La\u2066unch", "Lau\u200bnch", "\ufeffLaunch", "Launch\U000E0041", "Launch\x00", "La\u0085unch",
	} {
		t.Run(name, func(t *testing.T) {
			if err := Validate(name, goodBrief, ""); !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate(%q) = %v, want ErrInvalid", name, err)
			}
			if _, _, err := New(name, goodBrief, "", "/Hearth"); !errors.Is(err, ErrInvalid) {
				t.Errorf("New(%q) = %v, want ErrInvalid", name, err)
			}
		})
	}
	if err := Validate("Shop \U0001F469\u200d\U0001F4BB", goodBrief, ""); err != nil {
		t.Errorf("a ZWJ emoji sequence in a name = %v, want it allowed", err)
	}
}

func TestParts_AreStrippedOfInvisibleCharacters(t *testing.T) {
	dirty := Brief{
		Goal:          "Laun\u202ech",
		Context:       "We sell\u200b mugs.\n\tSee \U0001F469\u200d\U0001F4BB\U000E0041",
		DoneLooksLike: "\ufeffA live page\x00",
		DoneWhen:      "Orders\u2066 arrive\u0085",
	}
	want := Brief{
		Goal:          "Launch",
		Context:       "We sell mugs.\n\tSee \U0001F469\u200d\U0001F4BB",
		DoneLooksLike: "A live page",
		DoneWhen:      "Orders arrive",
	}
	p, _, err := New("Shop", dirty, "", "/Hearth")
	if err != nil || p.Brief != want {
		t.Errorf("New brief = %+v, %v; want %+v", p.Brief, err, want)
	}
	pr, err := Propose("Shop", dirty, "b", "c")
	if err != nil || pr.Brief != want {
		t.Errorf("Propose brief = %+v, %v; want %+v", pr.Brief, err, want)
	}
	e, err := Edit(p, "Shop", dirty, "")
	if err != nil || e.Brief != want {
		t.Errorf("Edit brief = %+v, %v; want %+v", e.Brief, err, want)
	}
	a, _, err := Activate(pr, "Shop", dirty, "", "/Hearth")
	if err != nil || a.Brief != want {
		t.Errorf("Activate brief = %+v, %v; want %+v", a.Brief, err, want)
	}
}

func TestParts_OfInvisibleCharactersAloneAreEmpty(t *testing.T) {
	b := goodBrief
	b.Goal = "\u200b\ufeff \u202e"
	if err := Validate("Shop", b, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("Validate = %v, want ErrInvalid", err)
	}
	if _, _, err := New("Shop", b, "", "/Hearth"); !errors.Is(err, ErrInvalid) {
		t.Errorf("New = %v, want ErrInvalid", err)
	}
}
