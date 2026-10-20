package ask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/widget"
	"github.com/rfizzle/hearth/internal/tools"
)

// The limits on a question in words, beside the control's own.
const (
	maxQuestion  = 1000
	maxSuggested = 5
)

const description = "Ask the owner a question and wait for the answer. Your turn ends here; the answer comes back as the result. " +
	"Ask only what you cannot decide yourself, one thing at a time. " +
	"Add widget when the answer is two to five concrete options, a date, a folder or file, a yes or no, or a rating: the owner answers with a click and you read exactly what they picked. " +
	"Leave widget out when the answer is open-ended or the owner would rather say it in their own words, and when unsure. " +
	"The kinds, with an example each: " +
	`single_choice, "Which draft should I send?" with options [{"id":"a","label":"The short one"},{"id":"b","label":"The long one"}]; ` +
	`multiple_choice, "Which sections should I keep?" with options and max_choices; ` +
	`free_text, "What should the title be?" with max_length 80; ` +
	`yes_no, "Shall I archive the old reports?" with yes_label "Archive them"; ` +
	`date, "When is the deadline?" with earliest "2026-10-21"; ` +
	`path, "Which folder holds the invoices?" with folders true; ` +
	`scale, "How formal should it sound?" with min 1, max 5, min_label "Chatty", max_label "Formal". ` +
	"If the request is malformed you are told what to change; fix it and ask again."

const parameters = `{"type":"object","properties":{` +
	`"question":{"type":"string","description":"The question, in one or two plain sentences the owner can answer without more context."},` +
	`"suggested_answers":{"type":"array","items":{"type":"string"},"description":"Up to five short replies the owner can pick instead of typing, for a question in words. Leave it out with a widget."},` +
	`"widget":{"type":"object","description":"One control to answer with, when a control beats words. Set kind and only that kind's settings.","properties":{` +
	`"kind":{"type":"string","enum":["single_choice","multiple_choice","free_text","yes_no","date","path","scale"]},` +
	`"required":{"type":"boolean","description":"True when the owner must use the control rather than reply in words."},` +
	`"options":{"type":"array","description":"single_choice: 2 to 8; multiple_choice: 2 to 12.","items":{"type":"object","properties":{` +
	`"id":{"type":"string","description":"A short unique id such as a."},"label":{"type":"string","description":"What the owner reads, 80 characters at most."},"detail":{"type":"string"}},"required":["id","label"]}},` +
	`"allow_other":{"type":"boolean","description":"single_choice: let the owner type their own answer."},` +
	`"min_choices":{"type":"integer"},"max_choices":{"type":"integer"},` +
	`"max_length":{"type":"integer","description":"free_text: at most 4000."},"placeholder":{"type":"string"},"multiline":{"type":"boolean"},` +
	`"yes_label":{"type":"string"},"no_label":{"type":"string"},` +
	`"earliest":{"type":"string","description":"date: YYYY-MM-DD."},"latest":{"type":"string","description":"date: YYYY-MM-DD."},"with_time":{"type":"boolean"},` +
	`"folders":{"type":"boolean"},"files":{"type":"boolean"},"start_path":{"type":"string"},` +
	`"min":{"type":"integer"},"max":{"type":"integer","description":"scale: at most ten steps from min."},"min_label":{"type":"string"},"max_label":{"type":"string"}` +
	`},"required":["kind"]}` +
	`},"required":["question"]}`

// Definition is ask_owner. It has no standing, so the dispatcher gates
// every call it does not refuse.
// See docs/model.md#owner-interaction.
func Definition() tools.Definition {
	return tools.Definition{
		Name:        string(org.AskOwner),
		Description: description,
		Parameters:  json.RawMessage(parameters),
		Capability:  org.AskOwner,
		Tier:        tools.TierAct,
		NoStanding:  true,
		Execute:     execute,
		Describe:    describe,
		Validate:    validate,
	}
}

// args is a call's arguments. Widget is kept raw so the catalogue parses it
// strictly.
type args struct {
	Question  string          `json:"question"`
	Suggested []string        `json:"suggested_answers"`
	Widget    json.RawMessage `json:"widget"`
}

// validate refuses a request the owner could not be shown as asked: no
// question, one too long, too many or empty suggested answers, a question
// or suggested answer with a line break or another control character, or a
// control the catalogue refuses. Each refusal says what to change.
func validate(raw json.RawMessage) error {
	var a args
	if err := json.Unmarshal(raw, &a); err != nil {
		return errors.New("the arguments are not a JSON object with a question")
	}
	if strings.TrimSpace(a.Question) == "" {
		return errors.New("question is required; write what you want to ask")
	}
	if n := utf8.RuneCountInString(a.Question); n > maxQuestion {
		return fmt.Errorf("the question is %d characters; keep it to %d or fewer", n, maxQuestion)
	}
	if err := widget.Plain("the question", a.Question); err != nil {
		return err
	}
	if err := validateSuggested(a.Suggested); err != nil {
		return err
	}
	if len(a.Widget) == 0 || string(a.Widget) == "null" {
		return nil
	}
	spec, err := widget.ParseSpec(a.Widget)
	if err != nil {
		return err
	}
	return widget.Validate(spec)
}

// validateSuggested refuses more suggested answers than the owner can scan,
// or one that is empty or longer than a label.
func validateSuggested(suggested []string) error {
	if len(suggested) > maxSuggested {
		return fmt.Errorf("there are %d suggested answers; give %d or fewer", len(suggested), maxSuggested)
	}
	for i, s := range suggested {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("suggested answer %d is empty", i+1)
		}
		if n := utf8.RuneCountInString(s); n > widget.MaxLabel {
			return fmt.Errorf("suggested answer %d is %d characters; keep each to %d or fewer", i+1, n, widget.MaxLabel)
		}
		if err := widget.Plain(fmt.Sprintf("suggested answer %d", i+1), s); err != nil {
			return err
		}
	}
	return nil
}

// errAsked is what the executor says of a valid call: it is answered by the
// owner, never run.
var errAsked = errors.New("ask_owner: the question waits for the owner and is never run as a tool")

// execute is never reached by a valid call, which the gate parks; it gives
// the refusal for one that is not.
func execute(_ context.Context, call tools.Call) (tools.Result, error) {
	if err := validate(call.Args); err != nil {
		return tools.Result{}, fmt.Errorf("ask_owner refused: %w", err)
	}
	return tools.Result{}, errAsked
}

// describe is the question as the owner's card reads it.
func describe(raw json.RawMessage) string {
	var a args
	_ = json.Unmarshal(raw, &a) // a describer never fails; an unreadable call asks nothing
	return "Ask you: " + strings.TrimSpace(a.Question)
}
