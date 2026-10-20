package widget_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org/widget"
)

func opts(n int) []widget.Option {
	out := make([]widget.Option, n)
	for i := range out {
		id := string(rune('a' + i))
		out[i] = widget.Option{ID: id, Label: "Label " + id}
	}
	return out
}

func single(n int) widget.Spec {
	return widget.Spec{Kind: widget.SingleChoice, Single: &widget.SingleSpec{Options: opts(n)}}
}

func multiple(n int, m widget.MultipleSpec) widget.Spec {
	m.Options = opts(n)
	return widget.Spec{Kind: widget.MultipleChoice, Multiple: &m}
}

func text(n int) widget.Spec {
	return widget.Spec{Kind: widget.FreeText, FreeText: &widget.FreeTextSpec{MaxLength: n}}
}

func date(d widget.DateSpec) widget.Spec { return widget.Spec{Kind: widget.Date, Date: &d} }

func scale(lo, hi int) widget.Spec {
	return widget.Spec{Kind: widget.Scale, Scale: &widget.ScaleSpec{Min: lo, Max: hi}}
}

func path(p widget.PathSpec) widget.Spec { return widget.Spec{Kind: widget.Path, Path: &p} }

func yesNo() widget.Spec { return widget.Spec{Kind: widget.YesNo, YesNo: &widget.YesNoSpec{}} }

func TestValidate(t *testing.T) {
	long := strings.Repeat("x", 81)
	dupe := single(2)
	dupe.Single.Options[1].ID = "a"
	empty := single(2)
	empty.Single.Options[0].ID = " "
	label := single(2)
	label.Single.Options[1].Label = long
	mixed := single(2)
	mixed.Scale = &widget.ScaleSpec{Min: 1, Max: 5}
	ph := text(0)
	ph.FreeText.Placeholder = long
	twoLines := single(2)
	twoLines.Single.Options[1].Label = "Keep it\n\nAlso delete everything"
	tabbed := single(2)
	tabbed.Single.Options[0].Detail = "a\tb"
	bell := text(0)
	bell.FreeText.Placeholder = "ring\x07"
	const oneLine = "write it on one line as plain text"

	cases := []struct {
		name string
		spec widget.Spec
		want string // empty means valid
	}{
		{"single 2 options", single(2), ""},
		{"single 8 options", single(8), ""},
		{"single 1 option", single(1), "a single choice needs 2 to 8 options; you gave 1"},
		{"single 9 options", single(9), "a single choice needs 2 to 8 options; you gave 9"},
		{"single empty id", empty, `option 1 has an empty id; give every option a short id such as "a"`},
		{"single dupe id", dupe, `options 1 and 2 share the id "a"; ids must be unique`},
		{"single long label", label, "the label of option 2 is 81 characters; keep labels to 80 or fewer"},
		{"single label on two lines", twoLines, "the label of option 2 holds a line break or another control character; " + oneLine},
		{"single detail with a tab", tabbed, "the detail of option 1 holds a line break or another control character; " + oneLine},
		{"text placeholder with a control character", bell, "the placeholder holds a line break or another control character; " + oneLine},
		{"yes no label with a delete character", widget.Spec{Kind: widget.YesNo, YesNo: &widget.YesNoSpec{YesLabel: "Yes\x7f"}},
			"yes_label holds a line break or another control character; " + oneLine},
		{"scale label with a carriage return", widget.Spec{Kind: widget.Scale, Scale: &widget.ScaleSpec{Min: 1, Max: 5, MaxLabel: "High\r"}},
			"max_label holds a line break or another control character; " + oneLine},
		{"single with scale fields", mixed, "a single choice carries the settings of exactly one control, its own; you gave single_choice and scale"},
		{"multiple 12 options", multiple(12, widget.MultipleSpec{MaxChoices: 12}), ""},
		{"multiple 13 options", multiple(13, widget.MultipleSpec{}), "a multiple choice needs 2 to 12 options; you gave 13"},
		{"multiple 1 option", multiple(1, widget.MultipleSpec{}), "a multiple choice needs 2 to 12 options; you gave 1"},
		{"multiple max over options", multiple(3, widget.MultipleSpec{MaxChoices: 4}), "max_choices is 4 but there are only 3 options"},
		{"multiple min over max", multiple(4, widget.MultipleSpec{MinChoices: 3, MaxChoices: 2}), "min_choices 3 is more than max_choices 2"},
		{"text default limit", text(0), ""},
		{"text at limit", text(4000), ""},
		{"text over limit", text(4001), "free text takes a max_length of 1 to 4000; you gave 4001"},
		{"text long placeholder", ph, "the placeholder is 81 characters; keep it to 80 or fewer"},
		{"yes no", yesNo(), ""},
		{"yes no long label", widget.Spec{Kind: widget.YesNo, YesNo: &widget.YesNoSpec{NoLabel: long}}, "no_label is 81 characters; keep it to 80 or fewer"},
		{"date open", date(widget.DateSpec{}), ""},
		{"date range", date(widget.DateSpec{Earliest: "2026-10-01", Latest: "2026-10-31"}), ""},
		{"date reversed", date(widget.DateSpec{Earliest: "2026-11-01", Latest: "2026-10-31"}), "earliest 2026-11-01 is after latest 2026-10-31; put the earlier day first"},
		{"date malformed", date(widget.DateSpec{Latest: "next week"}), `latest "next week" is not a day written YYYY-MM-DD`},
		{"path folders", path(widget.PathSpec{Folders: true}), ""},
		{"path neither", path(widget.PathSpec{}), "a file or folder pick needs folders, files or both set to true"},
		{"scale 1 to 5", scale(1, 5), ""},
		{"scale ten steps", scale(1, 10), ""},
		{"scale eleven steps", scale(0, 10), "a scale takes at most 10 steps; 0 to 10 is 11"},
		{"scale min not below max", scale(5, 5), "a scale needs min below max; you gave min 5 and max 5"},
		{"unknown kind", widget.Spec{Kind: "slider"}, `the kind "slider" is not one of the controls; use one of: single_choice, multiple_choice, free_text, yes_no, date, path, scale`},
		{"no kind settings", widget.Spec{Kind: widget.Date}, "a date carries the settings of exactly one control, its own; you gave none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := widget.Validate(tc.spec)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, widget.ErrInvalid) {
				t.Fatalf("Validate = %v, want ErrInvalid", err)
			}
			if err.Error() != tc.want {
				t.Errorf("Validate = %q, want %q", err, tc.want)
			}
		})
	}
}

func yes(b bool) *bool { return &b }
func num(n int) *int   { return &n }

func TestCheck(t *testing.T) {
	req := func(s widget.Spec) widget.Spec { s.Required = true; return s }
	other := single(2)
	other.Single.AllowOther = true
	bounded := multiple(4, widget.MultipleSpec{MinChoices: 1, MaxChoices: 2})
	timed := date(widget.DateSpec{Earliest: "2026-10-10", Latest: "2026-10-20", WithTime: true})
	ranged := date(widget.DateSpec{Earliest: "2026-10-10", Latest: "2026-10-20"})
	long := strings.Repeat("é", 11)

	cases := []struct {
		name string
		spec widget.Spec
		ans  widget.Answer
		want string
	}{
		{"single ok", single(3), widget.Answer{Kind: widget.SingleChoice, Choices: []string{"b"}}, ""},
		{"single other ok", other, widget.Answer{Kind: widget.SingleChoice, Other: "neither"}, ""},
		{"single wrong id", single(3), widget.Answer{Kind: widget.SingleChoice, Choices: []string{"z"}}, `"z" is not one of the option ids`},
		{"single two choices", single(3), widget.Answer{Kind: widget.SingleChoice, Choices: []string{"a", "b"}}, "a single choice takes one option; the answer has 2"},
		{"single other not allowed", single(3), widget.Answer{Kind: widget.SingleChoice, Other: "x"}, "this question does not take an Other answer"},
		{"single choice and other", other, widget.Answer{Kind: widget.SingleChoice, Choices: []string{"a"}, Other: "x"}, "a single choice takes one option or Other, not both"},
		{"single required empty", req(single(3)), widget.Answer{Kind: widget.SingleChoice}, "a single choice needs an answer and the control was left empty"},
		{"single declined", single(3), widget.Answer{Kind: widget.SingleChoice}, ""},
		{"wrong kind", single(3), widget.Answer{Kind: widget.YesNo, Yes: yes(true)}, `this question asks for a single choice but the answer is for "yes_no"`},
		{"multiple ok", bounded, widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "c"}}, ""},
		{"multiple too many", bounded, widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "b", "c"}}, "a multiple choice takes at most 2 choices; the answer has 3"},
		{"multiple twice", bounded, widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "a"}}, `"a" was chosen twice`},
		{"multiple min empty", bounded, widget.Answer{Kind: widget.MultipleChoice}, "a multiple choice needs an answer and the control was left empty"},
		{"multiple none allowed", multiple(3, widget.MultipleSpec{}), widget.Answer{Kind: widget.MultipleChoice}, ""},
		{"text ok", text(10), widget.Answer{Kind: widget.FreeText, Text: "hello"}, ""},
		{"text at limit", text(10), widget.Answer{Kind: widget.FreeText, Text: strings.Repeat("é", 10)}, ""},
		{"text over", text(10), widget.Answer{Kind: widget.FreeText, Text: long}, "the text is 11 characters; the limit is 10"},
		{"text required empty", req(text(10)), widget.Answer{Kind: widget.FreeText}, "free text needs an answer and the control was left empty"},
		{"yes", yesNo(), widget.Answer{Kind: widget.YesNo, Yes: yes(false)}, ""},
		{"date ok", ranged, widget.Answer{Kind: widget.Date, Date: "2026-10-20"}, ""},
		{"date before", ranged, widget.Answer{Kind: widget.Date, Date: "2026-10-09"}, "the date 2026-10-09 is before the earliest allowed, 2026-10-10"},
		{"date after", ranged, widget.Answer{Kind: widget.Date, Date: "2026-10-21"}, "the date 2026-10-21 is after the latest allowed, 2026-10-20"},
		{"date malformed", ranged, widget.Answer{Kind: widget.Date, Date: "tomorrow"}, `the date "tomorrow" is not written YYYY-MM-DD`},
		{"date with time ok", timed, widget.Answer{Kind: widget.Date, Date: "2026-10-20 23:59"}, ""},
		{"date time missing", timed, widget.Answer{Kind: widget.Date, Date: "2026-10-20"}, `the date "2026-10-20" is not written YYYY-MM-DD HH:MM`},
		{"path ok", path(widget.PathSpec{Files: true}), widget.Answer{Kind: widget.Path, Path: "/Users/me/a.txt"}, ""},
		{"path blank", path(widget.PathSpec{Files: true}), widget.Answer{Kind: widget.Path, Path: "  "}, "the path is empty"},
		{"path required empty", req(path(widget.PathSpec{Files: true})), widget.Answer{Kind: widget.Path}, "a file or folder needs an answer and the control was left empty"},
		{"scale ok", scale(1, 5), widget.Answer{Kind: widget.Scale, Value: num(5)}, ""},
		{"scale high", scale(1, 5), widget.Answer{Kind: widget.Scale, Value: num(6)}, "the value 6 is outside the scale 1 to 5"},
		{"scale low", scale(1, 5), widget.Answer{Kind: widget.Scale, Value: num(0)}, "the value 0 is outside the scale 1 to 5"},
		{"scale required empty", req(scale(1, 5)), widget.Answer{Kind: widget.Scale}, "a scale needs an answer and the control was left empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := widget.Check(tc.spec, tc.ans)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Check = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, widget.ErrInvalid) {
				t.Fatalf("Check = %v, want ErrInvalid", err)
			}
			if err.Error() != tc.want {
				t.Errorf("Check = %q, want %q", err, tc.want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	other := single(2)
	other.Single.AllowOther = true
	labelled := widget.Spec{Kind: widget.YesNo, YesNo: &widget.YesNoSpec{YesLabel: "Keep it", NoLabel: "Drop it"}}

	cases := []struct {
		name string
		spec widget.Spec
		ans  widget.Answer
		want string
	}{
		{"single", single(3), widget.Answer{Kind: widget.SingleChoice, Choices: []string{"b"}}, "The owner chose: Label b"},
		{"single other", other, widget.Answer{Kind: widget.SingleChoice, Other: "a third way"}, "The owner chose: Other: a third way"},
		{"multiple", multiple(3, widget.MultipleSpec{}), widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "c"}}, "The owner chose: Label a, Label c"},
		{"multiple two", multiple(3, widget.MultipleSpec{}), widget.Answer{Kind: widget.MultipleChoice, Choices: []string{"a", "b"}}, "The owner chose: Label a, Label b"},
		{"multiple none", multiple(3, widget.MultipleSpec{}), widget.Answer{Kind: widget.MultipleChoice}, "The owner chose none"},
		{"free text", text(0), widget.Answer{Kind: widget.FreeText, Text: "Tuesday works"}, "The owner answered: Tuesday works"},
		{"yes", yesNo(), widget.Answer{Kind: widget.YesNo, Yes: yes(true)}, "The owner said: Yes"},
		{"no", yesNo(), widget.Answer{Kind: widget.YesNo, Yes: yes(false)}, "The owner said: No"},
		{"yes label", labelled, widget.Answer{Kind: widget.YesNo, Yes: yes(true)}, "The owner said: Keep it"},
		{"no label", labelled, widget.Answer{Kind: widget.YesNo, Yes: yes(false)}, "The owner said: Drop it"},
		{"date", date(widget.DateSpec{}), widget.Answer{Kind: widget.Date, Date: "2026-10-24"}, "The owner picked the date: 2026-10-24"},
		{"date and time", date(widget.DateSpec{WithTime: true}), widget.Answer{Kind: widget.Date, Date: "2026-10-24 14:30"}, "The owner picked the date: 2026-10-24 14:30"},
		{"path", path(widget.PathSpec{Folders: true}), widget.Answer{Kind: widget.Path, Path: "/Users/me/Reports"}, "The owner picked: /Users/me/Reports"},
		{"scale", scale(1, 5), widget.Answer{Kind: widget.Scale, Value: num(4)}, "The owner rated it 4 of 5"},
		{"declined", single(2), widget.Answer{Kind: widget.SingleChoice}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := widget.Render(tc.spec, tc.ans); got != tc.want {
				t.Errorf("Render = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	specs := []widget.Spec{
		single(3),
		multiple(4, widget.MultipleSpec{MinChoices: 1, MaxChoices: 3}),
		{Kind: widget.FreeText, Required: true, FreeText: &widget.FreeTextSpec{MaxLength: 200, Placeholder: "a name", Multiline: true}},
		{Kind: widget.YesNo, YesNo: &widget.YesNoSpec{YesLabel: "Keep it", NoLabel: "Drop it"}},
		date(widget.DateSpec{Earliest: "2026-10-01", Latest: "2026-10-31", WithTime: true}),
		path(widget.PathSpec{Folders: true, Files: true, StartPath: "/Users/me"}),
		{Kind: widget.Scale, Scale: &widget.ScaleSpec{Min: 0, Max: 5, MinLabel: "No", MaxLabel: "Yes"}},
	}
	for _, s := range specs {
		t.Run("spec "+string(s.Kind), func(t *testing.T) {
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"kind":"`+string(s.Kind)+`"`) {
				t.Errorf("no kind discriminator in %s", data)
			}
			got, err := widget.ParseSpec(data)
			if err != nil {
				t.Fatalf("ParseSpec(%s) = %v", data, err)
			}
			if !reflect.DeepEqual(got, s) {
				t.Errorf("round trip of %s = %+v, want %+v", data, got, s)
			}
		})
	}

	answers := []widget.Answer{
		{Kind: widget.SingleChoice, Choices: []string{"a"}},
		{Kind: widget.MultipleChoice, Choices: []string{"a", "b"}, Other: "more"},
		{Kind: widget.FreeText, Text: "hello"},
		{Kind: widget.YesNo, Yes: yes(false)},
		{Kind: widget.Date, Date: "2026-10-24 14:30"},
		{Kind: widget.Path, Path: "/Users/me/a.txt"},
		{Kind: widget.Scale, Value: num(0)},
	}
	for _, a := range answers {
		t.Run("answer "+string(a.Kind), func(t *testing.T) {
			data, err := json.Marshal(a)
			if err != nil {
				t.Fatal(err)
			}
			got, err := widget.ParseAnswer(data)
			if err != nil {
				t.Fatalf("ParseAnswer(%s) = %v", data, err)
			}
			if !reflect.DeepEqual(got, a) {
				t.Errorf("round trip of %s = %+v, want %+v", data, got, a)
			}
		})
	}
}

func TestParseRefusals(t *testing.T) {
	if _, err := widget.ParseSpec([]byte(`{"kind":"single_choice","colour":"red"}`)); !errors.Is(err, widget.ErrInvalid) {
		t.Errorf("an unknown field = %v, want ErrInvalid", err)
	}
	if _, err := widget.ParseSpec([]byte(`not json`)); !errors.Is(err, widget.ErrInvalid) {
		t.Errorf("bad JSON = %v, want ErrInvalid", err)
	}
	s, err := widget.ParseSpec([]byte(`{"kind":"slider"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := widget.Validate(s); !errors.Is(err, widget.ErrInvalid) {
		t.Errorf("an unknown kind validated: %v", err)
	}
	if _, err := widget.ParseAnswer([]byte(`{"kind":"scale","value":"four"}`)); !errors.Is(err, widget.ErrInvalid) {
		t.Errorf("a bad answer = %v, want ErrInvalid", err)
	}
}

func TestPlain(t *testing.T) {
	for _, s := range []string{"", "Keep it", "Café, 2 €"} {
		if err := widget.Plain("x", s); err != nil {
			t.Errorf("Plain(%q) = %v, want nil", s, err)
		}
	}
	for _, s := range []string{"a\nb", "a\r", "\tb", "nul\x00", "del\x7f", "c1\u0085"} {
		if err := widget.Plain("the label", s); !errors.Is(err, widget.ErrInvalid) || !strings.HasPrefix(err.Error(), "the label holds") {
			t.Errorf("Plain(%q) = %v, want a refusal naming the label", s, err)
		}
	}
}

func TestKinds(t *testing.T) {
	want := []widget.Kind{"single_choice", "multiple_choice", "free_text", "yes_no", "date", "path", "scale"}
	if got := widget.Kinds(); !reflect.DeepEqual(got, want) {
		t.Errorf("Kinds = %v, want %v", got, want)
	}
	if !widget.Known("date") || widget.Known("slider") {
		t.Error("Known disagrees with the catalogue")
	}
}
