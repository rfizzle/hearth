package ask

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/widget"
	"github.com/rfizzle/hearth/internal/tools"
)

const choice = `{"kind":"single_choice","options":[{"id":"a","label":"Keep it"},{"id":"b","label":"Archive it"},{"id":"c","label":"Delete it"}]}`

func TestValidate(t *testing.T) {
	long := strings.Repeat("x", maxQuestion+1)
	cases := []struct {
		name, args string
		refusal    string // empty when the request is valid
	}{
		{"a question in words", `{"question":"Which draft?"}`, ""},
		{"with suggested answers", `{"question":"Which draft?","suggested_answers":["The short one","The long one"]}`, ""},
		{"a single choice of three", `{"question":"What should I do with it?","widget":` + choice + `}`, ""},
		{"free text", `{"question":"Title?","widget":{"kind":"free_text","max_length":80}}`, ""},
		{"a null widget is none", `{"question":"Which?","widget":null}`, ""},
		{"not an object", `[1]`, "not a JSON object"},
		{"no question", `{"widget":` + choice + `}`, "question is required"},
		{"a blank question", `{"question":"   "}`, "question is required"},
		{"a question too long", `{"question":"` + long + `"}`, "keep it to 1000"},
		{"too many suggested answers", `{"question":"Q","suggested_answers":["a","b","c","d","e","f"]}`, "6 suggested answers"},
		{"an empty suggested answer", `{"question":"Q","suggested_answers":["a",""]}`, "suggested answer 2 is empty"},
		{"a single choice of one", `{"question":"Q","widget":{"kind":"single_choice","options":[{"id":"a","label":"Only"}]}}`, "a single choice needs 2 to 8 options; you gave 1"},
		{"an unknown kind", `{"question":"Q","widget":{"kind":"slider"}}`, "is not one of the controls"},
		{"a field the catalogue does not know", `{"question":"Q","widget":{"kind":"yes_no","colour":"red"}}`, "colour"},
		{"a widget that is not an object", `{"question":"Q","widget":"yes"}`, "widget is not valid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(json.RawMessage(tc.args))
			if tc.refusal == "" {
				if err != nil {
					t.Fatalf("validate = %v, want valid", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("validate = %v, want a refusal containing %q", err, tc.refusal)
			}
		})
	}
}

func TestValidate_AWidgetRefusalIsTheCatalogues(t *testing.T) {
	err := validate(json.RawMessage(`{"question":"Q","widget":{"kind":"scale","min":5,"max":1}}`))
	if !errors.Is(err, widget.ErrInvalid) {
		t.Fatalf("validate = %v, want it to wrap widget.ErrInvalid", err)
	}
}

func TestDefinition(t *testing.T) {
	d := Definition()
	if d.Name != "ask_owner" || d.Capability != org.AskOwner || d.Tier != tools.TierAct || !d.NoStanding || d.Validate == nil {
		t.Fatalf("definition = %+v", d)
	}
	if d.Standing() {
		t.Fatal("ask_owner may be allowed for good")
	}
	var schema map[string]any
	if err := json.Unmarshal(d.Parameters, &schema); err != nil {
		t.Fatalf("parameters are not JSON: %v", err)
	}
	for _, k := range widget.Kinds() {
		if !strings.Contains(d.Description, string(k)+", ") || !strings.Contains(string(d.Parameters), `"`+string(k)+`"`) {
			t.Errorf("the description or the schema leaves out %s", k)
		}
	}
	if got := d.Describe(json.RawMessage(`{"question":" Keep the old reports? "}`)); got != "Ask you: Keep the old reports?" {
		t.Errorf("Describe = %q", got)
	}
	if got := d.Describe(json.RawMessage(`nope`)); got != "Ask you: " {
		t.Errorf("Describe of junk = %q", got)
	}
}

func TestExecute_OnlyRefuses(t *testing.T) {
	_, err := execute(context.Background(), tools.Call{Args: json.RawMessage(`{"question":"Q","widget":{"kind":"date","earliest":"x"}}`)})
	if err == nil || !strings.HasPrefix(err.Error(), "ask_owner refused: ") {
		t.Fatalf("execute of a malformed call = %v, want the refusal", err)
	}
	if _, err := execute(context.Background(), tools.Call{Args: json.RawMessage(`{"question":"Q"}`)}); !errors.Is(err, errAsked) {
		t.Fatalf("execute of a valid call = %v, want errAsked", err)
	}
}

func TestDispatcher_GatesEveryValidCall(t *testing.T) {
	reg, err := tools.NewRegistry(Definition())
	if err != nil {
		t.Fatal(err)
	}
	_, grants, _ := org.NewBot("Dana", "Analyst", "", org.TierFast, t.TempDir())
	d := tools.Dispatcher{Defs: reg, Grants: grants}
	if got := d.Tools(); len(got) != 1 || got[0].Name != "ask_owner" {
		t.Fatalf("Tools = %+v, want ask_owner offered to a new bot", got)
	}
	if v, err := d.Decide("ask_owner", json.RawMessage(`{"question":"Which?","widget":`+choice+`}`)); v != tools.Gated || err != nil {
		t.Fatalf("Decide = %d, %v, want Gated", v, err)
	}
	_, err = d.Execute(context.Background(), "ask_owner", json.RawMessage(`{"question":"Q","widget":{"kind":"single_choice","options":[{"id":"a","label":"A"}]}}`))
	if want := "ask_owner refused: a single choice needs 2 to 8 options; you gave 1"; err == nil || err.Error() != want {
		t.Fatalf("Execute = %v, want %q", err, want)
	}
}
