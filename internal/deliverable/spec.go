package deliverable

import "errors"

// MaxBytes is the largest spec Parse reads.
const MaxBytes = 256 << 10

// The two kinds of deliverable. A report prints portrait and defaults to one
// column, a dashboard prints landscape and defaults to two.
const (
	Report    = "report"
	Dashboard = "dashboard"
)

// Kind names a widget. The names are the wire form and are never renamed.
type Kind string

// The ten kinds of widget.
const (
	Metric   Kind = "metric"
	Line     Kind = "line"
	Bar      Kind = "bar"
	Pie      Kind = "pie"
	Table    Kind = "table"
	List     Kind = "list"
	Progress Kind = "progress"
	Light    Kind = "light"
	Callout  Kind = "callout"
	Text     Kind = "text"
)

// entry is one row of the catalogue: the widget's name and the fields it takes
// beside kind, span and as_of.
type entry struct {
	kind   Kind
	fields string
}

const lineFields = "title categories series format y_label y_min y_max"

var catalogue = []entry{
	{Metric, "label value format decimals unit delta caption measure"},
	{Line, lineFields},
	{Bar, lineFields + " stacked"},
	{Pie, "title slices format"},
	{Table, "title columns rows footer"},
	{List, "title style items"},
	{Progress, "label done total caption measure"},
	{Light, "label tone word caption measure"},
	{Callout, "tone title markdown"},
	{Text, "markdown"},
}

// Kinds lists the ten kinds in catalogue order.
func Kinds() []Kind {
	ks := make([]Kind, len(catalogue))
	for i, e := range catalogue {
		ks[i] = e.kind
	}
	return ks
}

// ErrInvalid is wrapped by every refusal in this package.
var ErrInvalid = errors.New("invalid deliverable")

// invalid is a refusal in words; it matches ErrInvalid.
type invalid string

func (e invalid) Error() string        { return string(e) }
func (e invalid) Is(target error) bool { return target == ErrInvalid }

// Spec is a whole deliverable. Template is set by the core from the tool's
// template argument, never by a bot.
type Spec struct {
	Version  int       `json:"version"`
	Kind     string    `json:"kind"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle,omitempty"`
	Template string    `json:"template,omitempty"`
	Sections []Section `json:"sections"`
}

// Section is a grid of widgets. Columns 0 means the kind's default.
type Section struct {
	Heading string   `json:"heading,omitempty"`
	Columns int      `json:"columns,omitempty"`
	Widgets []Widget `json:"widgets"`
}

// Widget is one of the ten kinds in the flat JSON the wire uses, kind as the
// discriminator. Each kind takes some of the fields below and Parse refuses
// the rest; Span 0 means one column.
type Widget struct {
	Kind       Kind     `json:"kind"`
	Span       int      `json:"span,omitempty"`
	AsOf       string   `json:"as_of,omitempty"`
	Measure    string   `json:"measure,omitempty"`
	Label      string   `json:"label,omitempty"`
	Title      string   `json:"title,omitempty"`
	Caption    string   `json:"caption,omitempty"`
	Value      *float64 `json:"value,omitempty"`
	Format     string   `json:"format,omitempty"`
	Decimals   *int     `json:"decimals,omitempty"`
	Unit       string   `json:"unit,omitempty"`
	Delta      *Delta   `json:"delta,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Series     []Series `json:"series,omitempty"`
	YLabel     string   `json:"y_label,omitempty"`
	YMin       *float64 `json:"y_min,omitempty"`
	YMax       *float64 `json:"y_max,omitempty"`
	Stacked    bool     `json:"stacked,omitempty"`
	Slices     []Slice  `json:"slices,omitempty"`
	Columns    []Column `json:"columns,omitempty"`
	Rows       [][]any  `json:"rows,omitempty"`
	Footer     []any    `json:"footer,omitempty"`
	Style      string   `json:"style,omitempty"`
	Items      []Item   `json:"items,omitempty"`
	Done       *float64 `json:"done,omitempty"`
	Total      *float64 `json:"total,omitempty"`
	Tone       string   `json:"tone,omitempty"`
	Word       string   `json:"word,omitempty"`
	Markdown   string   `json:"markdown,omitempty"`
}

// Delta is the change a metric shows beside its value; Good says whether the
// change is a good one, so a falling cost can read as good.
type Delta struct {
	Value float64 `json:"value"`
	Good  *bool   `json:"good,omitempty"`
}

// Series is one named run of values, one per category.
type Series struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// Slice is one wedge of a pie.
type Slice struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// Column heads a table column; Align is left, center or right.
type Column struct {
	Header string `json:"header"`
	Align  string `json:"align,omitempty"`
	Format string `json:"format,omitempty"`
}

// Item is a line of a list; Done is read only in a check list.
type Item struct {
	Text   string `json:"text"`
	Done   bool   `json:"done,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Value is what the measure layer reads for one bound widget: a number, a
// progress or a light, with the time it was read.
type Value struct {
	ID      string
	Kind    string // number, progress or light
	Label   string
	Number  float64
	Unit    string
	Done    float64
	Total   float64
	Tone    string
	Word    string
	AsOf    string
	Missing string
}
