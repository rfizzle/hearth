package park

import (
	"errors"
	"testing"

	"github.com/rfizzle/hearth/internal/org/widget"
)

func TestAnswer(t *testing.T) {
	const (
		single   = `{"kind":"single_choice","options":[{"id":"a","label":"Keep it"},{"id":"b","label":"Archive it"},{"id":"c","label":"Delete it"}]}`
		required = `{"kind":"single_choice","required":true,"options":[{"id":"a","label":"Keep it"},{"id":"b","label":"Archive it"}]}`
		text     = `{"kind":"free_text"}`
		multi    = `{"kind":"multiple_choice","options":[{"id":"a","label":"Intro"},{"id":"b","label":"Costs"}]}`
		scale    = `{"kind":"scale","min":1,"max":5}`
	)
	four := 4
	cases := []struct {
		name, spec, text string
		value            *widget.Answer
		want             Reply
		err              error
	}{
		{"words to a question in words", "", " Tuesday works ", nil,
			Reply{Owner: "Tuesday works", Result: "The owner answered: Tuesday works"}, nil},
		{"a choice", single, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"b"}},
			Reply{Owner: "Archive it", Result: "The owner chose: Archive it", Value: `{"kind":"single_choice","choices":["b"]}`}, nil},
		{"a choice and words", single, "but keep a copy", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"b"}},
			Reply{Owner: "Archive it\n\nbut keep a copy", Result: "The owner chose: Archive it\n\nThe owner answered: but keep a copy",
				Value: `{"kind":"single_choice","choices":["b"]}`}, nil},
		{"words instead of a control", single, "none of these", nil,
			Reply{Owner: "none of these", Result: "The owner answered: none of these"}, nil},
		{"free text in the control", text, "", &widget.Answer{Kind: widget.FreeText, Text: "Q3 report"},
			Reply{Owner: "Q3 report", Result: "The owner answered: Q3 report", Value: `{"kind":"free_text","text":"Q3 report"}`}, nil},
		{"free text sent twice reads once", text, "Q3 report", &widget.Answer{Kind: widget.FreeText, Text: "Q3 report"},
			Reply{Owner: "Q3 report", Result: "The owner answered: Q3 report", Value: `{"kind":"free_text","text":"Q3 report"}`}, nil},
		{"none of several", multi, "", &widget.Answer{Kind: widget.MultipleChoice},
			Reply{Owner: "None", Result: "The owner chose none", Value: `{"kind":"multiple_choice"}`}, nil},
		{"a rating", scale, "", &widget.Answer{Kind: widget.Scale, Value: &four},
			Reply{Owner: "4 of 5", Result: "The owner rated it 4 of 5", Value: `{"kind":"scale","value":4}`}, nil},
		{"an unknown choice", single, "", &widget.Answer{Kind: widget.SingleChoice, Choices: []string{"z"}}, Reply{}, widget.ErrInvalid},
		{"the wrong kind", single, "", &widget.Answer{Kind: widget.YesNo}, Reply{}, widget.ErrInvalid},
		{"words to a required control", required, "maybe", nil, Reply{}, widget.ErrInvalid},
		{"a control's answer to a question in words", "", "", &widget.Answer{Kind: widget.YesNo}, Reply{}, errNoControl},
		{"nothing at all", "", "  ", nil, Reply{}, ErrEmpty},
		{"an empty control and no words", single, "", &widget.Answer{Kind: widget.SingleChoice}, Reply{}, ErrEmpty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Answer(tc.spec, tc.text, tc.value)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("Answer = %+v, %v, want %v", got, err, tc.err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("Answer = %+v, %v\nwant     %+v", got, err, tc.want)
			}
		})
	}
}
