package widget

import "errors"

// Kind names a control. The names are the wire form and are never renamed.
type Kind string

// The seven kinds of control.
const (
	SingleChoice   Kind = "single_choice"
	MultipleChoice Kind = "multiple_choice"
	FreeText       Kind = "free_text"
	YesNo          Kind = "yes_no"
	Date           Kind = "date"
	Path           Kind = "path"
	Scale          Kind = "scale"
)

// The limits the catalogue holds a Spec and an Answer to.
const (
	MaxLabel       = 80
	MaxPlaceholder = 80
	MaxText        = 4000
	DefaultText    = 1000
	MaxOther       = 500
	MaxPath        = 4096
	MaxScaleSteps  = 10
)

// DateLayout and DateTimeLayout are the forms a date answer takes.
const (
	DateLayout     = "2006-01-02"
	DateTimeLayout = "2006-01-02 15:04"
)

// ErrInvalid is wrapped by every refusal in this package.
var ErrInvalid = errors.New("invalid widget")

// entry is one row of the catalogue.
type entry struct {
	kind Kind
	noun string
}

// catalogue is the closed set, in the order the framing lists it.
var catalogue = []entry{
	{SingleChoice, "a single choice"},
	{MultipleChoice, "a multiple choice"},
	{FreeText, "free text"},
	{YesNo, "a yes or no"},
	{Date, "a date"},
	{Path, "a file or folder"},
	{Scale, "a scale"},
}

// optionBounds is how many options each choice kind takes, fewest then most.
var optionBounds = map[Kind][2]int{
	SingleChoice:   {2, 8},
	MultipleChoice: {2, 12},
}

// Kinds lists the seven kinds in catalogue order.
func Kinds() []Kind {
	out := make([]Kind, len(catalogue))
	for i, e := range catalogue {
		out[i] = e.kind
	}
	return out
}

// Known reports whether k is one of the seven kinds.
func Known(k Kind) bool {
	_, ok := lookup(k)
	return ok
}

func lookup(k Kind) (entry, bool) {
	for _, e := range catalogue {
		if e.kind == k {
			return e, true
		}
	}
	return entry{}, false
}

// Option is one choice: a short id the answer names and a label the owner
// reads.
type Option struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
}

// SingleSpec asks the owner to pick one option.
type SingleSpec struct {
	Options    []Option `json:"-"`
	AllowOther bool     `json:"allow_other,omitempty"`
}

// MultipleSpec asks the owner to pick several options. MinChoices and
// MaxChoices bound the count; a zero MaxChoices means no bound beyond the
// options themselves.
type MultipleSpec struct {
	Options    []Option `json:"-"`
	MinChoices int      `json:"min_choices,omitempty"`
	MaxChoices int      `json:"max_choices,omitempty"`
}

// FreeTextSpec asks for words. A zero MaxLength means DefaultText.
type FreeTextSpec struct {
	MaxLength   int    `json:"max_length,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Multiline   bool   `json:"multiline,omitempty"`
}

// YesNoSpec asks a yes or no. Empty labels read "Yes" and "No".
type YesNoSpec struct {
	YesLabel string `json:"yes_label,omitempty"`
	NoLabel  string `json:"no_label,omitempty"`
}

// DateSpec asks for a day, and a time of day when WithTime is set. Earliest
// and Latest are optional DateLayout days that bound the answer.
type DateSpec struct {
	Earliest string `json:"earliest,omitempty"`
	Latest   string `json:"latest,omitempty"`
	WithTime bool   `json:"with_time,omitempty"`
}

// PathSpec asks for a folder, a file or either; at least one is set.
// StartPath is a hint the panel opens at.
type PathSpec struct {
	Folders   bool   `json:"folders,omitempty"`
	Files     bool   `json:"files,omitempty"`
	StartPath string `json:"start_path,omitempty"`
}

// ScaleSpec asks for a whole number from Min to Max.
type ScaleSpec struct {
	Min      int    `json:"min,omitempty"`
	Max      int    `json:"max,omitempty"`
	MinLabel string `json:"min_label,omitempty"`
	MaxLabel string `json:"max_label,omitempty"`
}

// Spec is one control. Exactly the field that matches Kind is set.
type Spec struct {
	Kind     Kind
	Required bool

	Single   *SingleSpec
	Multiple *MultipleSpec
	FreeText *FreeTextSpec
	YesNo    *YesNoSpec
	Date     *DateSpec
	Path     *PathSpec
	Scale    *ScaleSpec
}

// Answer is what the owner gave. Which fields are used follows the Spec's
// Kind; an Answer with none set is a control left empty.
type Answer struct {
	Kind    Kind     `json:"kind"`
	Choices []string `json:"choices,omitempty"`
	Other   string   `json:"other,omitempty"`
	Text    string   `json:"text,omitempty"`
	Yes     *bool    `json:"yes,omitempty"`
	Date    string   `json:"date,omitempty"`
	Path    string   `json:"path,omitempty"`
	Value   *int     `json:"value,omitempty"`
}

// empty reports whether the answer carries no value.
func (a Answer) empty() bool {
	return len(a.Choices) == 0 && a.Other == "" && a.Text == "" && a.Yes == nil &&
		a.Date == "" && a.Path == "" && a.Value == nil
}

// invalid is a refusal in words; it matches ErrInvalid.
type invalid string

func (e invalid) Error() string        { return string(e) }
func (e invalid) Is(target error) bool { return target == ErrInvalid }
