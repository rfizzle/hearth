package template

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/deliverable"
)

// refusal is a template refused in words; it matches deliverable.ErrInvalid.
type refusal string

func (e refusal) Error() string        { return string(e) }
func (e refusal) Is(target error) bool { return target == deliverable.ErrInvalid }

func refuse(format string, args ...any) error {
	return refusal(fmt.Sprintf(format, args...))
}

// Template is a spec with a name, a sentence on when to use it, and per section
// the widget kinds it takes. Sample is the spec with its example widgets, so a
// template renders as itself in a gallery.
type Template struct {
	ID      string
	Name    string
	When    string
	Shipped bool
	Slots   []Slot // one per section of Sample
	Sample  deliverable.Spec
}

// Slot says what one section of a template takes.
type Slot struct {
	Allow []deliverable.Kind
	Hint  string
}

// Parse reads a template file: a spec of the ordinary schema with a template
// object (id, name, when) and, per section, allow and hint. It refuses what
// deliverable.Parse and Validate refuse, a malformed header, and an example
// widget its own section does not allow.
func Parse(data []byte) (Template, error) {
	if len(data) > deliverable.MaxBytes {
		return Template{}, refuse("the template is %d KB; the limit is %d KB", (len(data)+1023)/1024, deliverable.MaxBytes>>10)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return Template{}, refuse("the template is not valid JSON: %v", err)
	}
	var head struct{ ID, Name, When string }
	if err := decodeKey(top, "template", &head, `{"id","name","when"}`); err != nil {
		return Template{}, err
	}
	var raw []map[string]json.RawMessage
	if err := decodeKey(top, "sections", &raw, "a list of sections"); err != nil {
		return Template{}, err
	}
	slots := make([]Slot, len(raw))
	for i, sec := range raw {
		var allow []deliverable.Kind
		if err := decodeKey(sec, "allow", &allow, "a list of widget kinds"); err != nil {
			return Template{}, prefix(i, err)
		}
		if err := decodeKey(sec, "hint", &slots[i].Hint, "text"); err != nil {
			return Template{}, prefix(i, err)
		}
		slots[i].Allow = allow
	}
	spec, err := deliverable.Parse(rebuilt(top, raw))
	if err != nil {
		return Template{}, err
	}
	spec.Template = head.ID
	t := Template{ID: head.ID, Name: head.Name, When: head.When, Slots: slots, Sample: spec}
	return t, t.check()
}

// decodeKey reads m[key] into v and removes it, so what is left is the spec.
func decodeKey(m map[string]json.RawMessage, key string, v any, shape string) error {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	delete(m, key)
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return refuse("the %s of the template should be %s: %v", key, shape, err)
	}
	return nil
}

// rebuilt is the spec JSON left when the template-only parts are taken out.
func rebuilt(top map[string]json.RawMessage, sections []map[string]json.RawMessage) []byte {
	if sections == nil {
		sections = []map[string]json.RawMessage{}
	}
	top["sections"], _ = json.Marshal(sections)
	b, _ := json.Marshal(top)
	return b
}

func prefix(i int, err error) error {
	return refuse("section %d: %v", i+1, err)
}

// check refuses a header or a slot the rules do not allow, and a sample that
// does not validate.
func (t Template) check() error {
	if n := len(t.ID); n < 1 || n > 40 || strings.ContainsFunc(t.ID, idRuneNot) {
		return refuse("the template id %q is 1 to 40 letters a to z, digits and hyphens", t.ID)
	}
	if err := words("name", t.Name, 60); err != nil {
		return err
	}
	if err := words("when", t.When, 200); err != nil {
		return err
	}
	if err := deliverable.Validate(t.Sample); err != nil {
		return refuse("the template %s does not hold together: %v", t.ID, err)
	}
	for i, slot := range t.Slots {
		if err := t.checkSlot(i, slot); err != nil {
			return err
		}
	}
	return nil
}

// idRuneNot is true for a rune an id may not hold.
func idRuneNot(r rune) bool {
	return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-'
}

func words(what, s string, hi int) error {
	if n := utf8.RuneCountInString(s); n < 1 || n > hi || strings.ContainsFunc(s, unicode.IsControl) {
		return refuse("the template %s is 1 to %d characters on one line", what, hi)
	}
	return nil
}

func (t Template) checkSlot(i int, slot Slot) error {
	if n := len(slot.Allow); n < 1 || n > 10 {
		return refuse("section %d allows %d kinds; give 1 to 10", i+1, len(slot.Allow))
	}
	known := deliverable.Kinds()
	for _, k := range slot.Allow {
		if !slices.Contains(known, k) {
			return refuse("section %d allows %q, which is not one of the widgets", i+1, k)
		}
	}
	if utf8.RuneCountInString(slot.Hint) > 200 || strings.ContainsFunc(slot.Hint, unicode.IsControl) {
		return refuse("the hint of section %d is up to 200 characters on one line", i+1)
	}
	for _, w := range t.Sample.Sections[i].Widgets {
		if !slices.Contains(slot.Allow, w.Kind) {
			return refuse("the example in section %d is a %s, which its section does not allow", i+1, w.Kind)
		}
	}
	return nil
}

// Apply makes the Spec a bot's filled sections ask for. The kind is the
// template's, the sections are given in order and there are as many as the
// template has, their headings and columns are the template's (a bot that
// supplies them is refused), and each widget is of a kind its section allows.
// The result carries the template's id and has passed Validate.
func Apply(t Template, filled deliverable.Spec) (deliverable.Spec, error) {
	if err := deliverable.Authored(filled); err != nil {
		return deliverable.Spec{}, err
	}
	if filled.Kind != t.Sample.Kind {
		return deliverable.Spec{}, refuse("the template %s makes a %s; the spec says %q", t.ID, t.Sample.Kind, filled.Kind)
	}
	if len(filled.Sections) != len(t.Slots) {
		return deliverable.Spec{}, refuse("%s has %d sections; the spec has %d; give one for each, in order", t.ID, len(t.Slots), len(filled.Sections))
	}
	out := filled
	out.Template = t.ID
	out.Sections = slices.Clone(filled.Sections)
	for i, sec := range filled.Sections {
		if err := t.fit(i, sec); err != nil {
			return deliverable.Spec{}, err
		}
		out.Sections[i].Heading = t.Sample.Sections[i].Heading
		out.Sections[i].Columns = t.Sample.Sections[i].Columns
	}
	if err := deliverable.Validate(out); err != nil {
		return deliverable.Spec{}, err
	}
	return out, nil
}

// fit refuses a filled section that supplies what the template fixes or holds a
// widget its slot does not allow.
func (t Template) fit(i int, sec deliverable.Section) error {
	switch {
	case sec.Heading != "":
		return refuse("section %d's heading comes from the template; leave it out", i+1)
	case sec.Columns != 0:
		return refuse("section %d's columns come from the template; leave them out", i+1)
	}
	allow := t.Slots[i].Allow
	for _, w := range sec.Widgets {
		if !slices.Contains(allow, w.Kind) {
			return refuse("section %q of %s takes %s; it was given a %s", t.Sample.Sections[i].Heading, t.ID, orList(allow), w.Kind)
		}
	}
	return nil
}

func orList(ks []deliverable.Kind) string {
	names := make([]string, len(ks))
	for i, k := range ks {
		names[i] = string(k)
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}
