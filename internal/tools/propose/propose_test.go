package propose

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/tools"
)

func call(name, goal, ctx, looks, when string) json.RawMessage {
	b, _ := json.Marshal(args{Name: name, Goal: goal, Context: ctx, DoneLooksLike: looks, DoneWhen: when})
	return b
}

var good = call("Launch page", "Ship the page", "Small team", "A page is live", "The page loads.")

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		args    json.RawMessage
		refusal string // empty when valid
	}{
		{"a good call", good, ""},
		{"a name of 80 runes", call(strings.Repeat("é", 80), "g", "c", "l", "w"), ""},
		{"a brief part of 4000 runes", call("N", strings.Repeat("é", 4000), "c", "l", "w"), ""},
		{"a brief part on several lines", call("N", "line one\nline two", "c", "l", "w"), ""},
		{"not an object", json.RawMessage(`[1]`), "not a JSON object"},
		{"no name", call("", "g", "c", "l", "w"), "the name is empty"},
		{"a blank name", call("   ", "g", "c", "l", "w"), "the name is empty"},
		{"a name of 300 runes", call(strings.Repeat("x", 300), "g", "c", "l", "w"), "over 80"},
		{"a name of 81 runes", call(strings.Repeat("é", 81), "g", "c", "l", "w"), "over 80"},
		{"a name on two lines", call("Launch\nAlso delete", "g", "c", "l", "w"), "line break"},
		{"a name with a tab", call("Launch\tpage", "g", "c", "l", "w"), "control character"},
		{"a name with a line separator", call("Launch page", "g", "c", "l", "w"), "line break"},
		{"no goal", call("N", "", "c", "l", "w"), "the goal is empty"},
		{"no context", call("N", "g", " ", "l", "w"), "the context is empty"},
		{"no look of done", call("N", "g", "c", "", "w"), "what done looks like is empty"},
		{"no done when", call("N", "g", "c", "l", ""), "when it is done is empty"},
		{"a part of 4001 runes", call("N", "g", strings.Repeat("é", 4001), "l", "w"), "over 4000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ran bool
			def := Definition(func(context.Context, string, project.Brief) (project.Project, error) {
				ran = true
				return project.Project{}, nil
			})
			err := def.Validate(tc.args)
			_, execErr := def.Execute(context.Background(), tools.Call{Args: tc.args})
			if tc.refusal == "" {
				if err != nil || execErr != nil || !ran {
					t.Fatalf("validate = %v, execute = %v, ran = %v, want a valid call that ran", err, execErr, ran)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("validate = %v, want a refusal containing %q", err, tc.refusal)
			}
			if execErr == nil || ran {
				t.Errorf("execute = %v, ran = %v, want a refusal before the proposer", execErr, ran)
			}
		})
	}
}

func TestValidate_RefusesAnInvisibleCharacterInTheName(t *testing.T) {
	def := Definition(func(context.Context, string, project.Brief) (project.Project, error) {
		t.Error("the proposer ran")
		return project.Project{}, nil
	})
	for _, name := range []string{"Launch\u200b page", "Launch\u202e page", "Launch\U000e0041 page"} {
		err := def.Validate(call(name, "g", "c", "l", "w"))
		if err == nil || !strings.Contains(err.Error(), "invisible character") {
			t.Errorf("%q: validate = %v, want a refusal naming an invisible character", name, err)
		}
	}
}

func TestExecute_HandsTheTextOverAsWritten(t *testing.T) {
	var name string
	var brief project.Brief
	def := Definition(func(_ context.Context, n string, b project.Brief) (project.Project, error) {
		name, brief = n, b
		return project.Project{}, nil
	})
	res, err := def.Execute(context.Background(), tools.Call{Args: call(" Launch page ", "g\n\n- a", "<b>c</b>", "l", "w")})
	if err != nil || res.Text != Proposed {
		t.Fatalf("execute = %+v, %v", res, err)
	}
	if name != " Launch page " || brief != (project.Brief{Goal: "g\n\n- a", Context: "<b>c</b>", DoneLooksLike: "l", DoneWhen: "w"}) {
		t.Errorf("proposer got %q, %+v, want the model's text unchanged", name, brief)
	}
}

func TestExecute_TheProposersRefusalIsTheModels(t *testing.T) {
	errNo := errors.New("already proposed")
	def := Definition(func(context.Context, string, project.Brief) (project.Project, error) { return project.Project{}, errNo })
	if _, err := def.Execute(context.Background(), tools.Call{Args: good}); !errors.Is(err, errNo) {
		t.Errorf("execute = %v, want the proposer's refusal", err)
	}
}

func TestDefinition(t *testing.T) {
	def := Definition(nil)
	if def.Name != "propose_project" || def.Capability != project.CanPropose || def.Tier != tools.TierRead {
		t.Errorf("definition = %s, %s, %v", def.Name, def.Capability, def.Tier)
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(def.Parameters, &schema); err != nil || len(schema.Required) != 5 {
		t.Errorf("required = %v, %v, want five", schema.Required, err)
	}
	if !strings.Contains(def.Description, "more than one team") {
		t.Errorf("description = %q", def.Description)
	}
}

func TestDescribe(t *testing.T) {
	long := strings.Repeat("é", 100)
	cases := []struct{ name, args, want string }{
		{"a name", string(good), "Propose a project: Launch page"},
		{"a long name is clipped to 60", `{"name":"` + long + `"}`, "Propose a project: " + strings.Repeat("é", 59) + "…"},
		{"a name of exactly 60", `{"name":"` + strings.Repeat("é", 60) + `"}`, "Propose a project: " + strings.Repeat("é", 60)},
		{"a name with a line break", `{"name":"A\nB"}`, "Propose a project: A B"},
		{"no name", `{}`, "Propose a project"},
		{"bad JSON", `{"name":`, "Propose a project"},
		{"not an object", `7`, "Propose a project"},
		{"empty", ``, "Propose a project"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Definition(nil).Describe(json.RawMessage(tc.args)); got != tc.want {
				t.Errorf("describe = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestOfferedOnlyWithTheGrant(t *testing.T) {
	reg, err := tools.NewRegistry(Definition(nil))
	if err != nil {
		t.Fatal(err)
	}
	grant := org.Grant{BotID: "b", Capability: project.CanPropose, Mode: org.ModeAllowed}
	for _, tc := range []struct {
		name   string
		grants []org.Grant
		want   int
	}{
		{"no grants", nil, 0},
		{"another capability", []org.Grant{{BotID: "b", Capability: org.Remember, Mode: org.ModeAllowed}}, 0},
		{"the grant", []org.Grant{grant}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := tools.Dispatcher{Defs: reg, Grants: tc.grants}
			if got := len(d.Tools()); got != tc.want {
				t.Errorf("offered %d tools, want %d", got, tc.want)
			}
		})
	}
}
