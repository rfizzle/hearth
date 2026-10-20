package park

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/rfizzle/hearth/internal/org/widget"
)

// Reply is what the owner's answer to a question comes to: Owner, their
// message in the thread in their own words; Result, the fixed words the bot
// reads as the call's result; and Value, the JSON of their answer to the
// control, empty when they answered in words alone.
type Reply struct {
	Owner  string
	Result string
	Value  string
}

// ErrEmpty is an answer that says nothing.
var ErrEmpty = errors.New("the answer is empty")

// errNoControl refuses a control's answer to a question asked in words.
var errNoControl = errors.New("this question has no control to answer; answer it in words")

// Answer is the reply the owner gave to a question asked with the control
// spec, the JSON a question's call carried, or none: text in words, value to
// the control, or both. A control they left empty and answered in words is
// declined unless it is required; any answer to the control is checked
// against it. The bot reads the control's answer in the catalogue's fixed
// words, and words typed beside it after them, never the answer as data.
// See docs/model.md#owner-interaction.
func Answer(spec, text string, value *widget.Answer) (Reply, error) {
	text = strings.TrimSpace(text)
	if spec == "" && value != nil {
		return Reply{}, errNoControl
	}
	if spec == "" {
		return inWords(text)
	}
	s, err := widget.ParseSpec([]byte(spec))
	if err != nil {
		return Reply{}, err
	}
	if value == nil && text != "" && !s.Required {
		return inWords(text)
	}
	return toControl(s, text, value)
}

// toControl is the reply that answers the control s with value, or leaves
// it empty, with any words typed beside it.
func toControl(s widget.Spec, text string, value *widget.Answer) (Reply, error) {
	a := widget.Answer{Kind: s.Kind}
	if value != nil {
		a = *value
	}
	if err := widget.Check(s, a); err != nil {
		return Reply{}, err
	}
	rendered := widget.Render(s, a)
	r := Reply{Owner: ownerWords(rendered), Result: rendered}
	if text != "" && text != a.Text {
		r.Owner, r.Result = join(r.Owner, text), join(r.Result, Answered(text))
	}
	if r.Result == "" {
		return Reply{}, ErrEmpty
	}
	b, err := json.Marshal(a)
	r.Value = string(b)
	return r, err
}

// inWords is a reply typed in the thread.
func inWords(text string) (Reply, error) {
	if text == "" {
		return Reply{}, ErrEmpty
	}
	return Reply{Owner: text, Result: Answered(text)}, nil
}

// ownerWords is a rendered answer as the owner would say it: the choice
// without the catalogue's fixed words before it, as "Keep it" or "4 of 5".
func ownerWords(rendered string) string {
	if rendered == "The owner chose none" {
		return "None"
	}
	for _, p := range []string{
		"The owner chose: ", "The owner answered: ", "The owner said: ",
		"The owner picked the date: ", "The owner picked: ", "The owner rated it ",
	} {
		if rest, ok := strings.CutPrefix(rendered, p); ok {
			return rest
		}
	}
	return rendered
}

// join puts b after a, with a blank line between when both are there.
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}
