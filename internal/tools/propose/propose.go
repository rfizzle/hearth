package propose

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/tools"
)

// maxDescribed is the most runes of a project's name the owner's words for a
// call carry.
const maxDescribed = 60

const description = "Propose a project when the person's request clearly spans more than one team or will take more than one round of work. " +
	"Give it a short name and a brief in four parts: the goal, the context, what done looks like, and one \"done when\" sentence. " +
	"The person sees a card and decides; nothing is assigned until they confirm. " +
	"Do not propose a project for a request you can answer directly or that one team can finish in one go."

const parameters = `{"type":"object","properties":{` +
	`"name":{"type":"string","description":"A short name for the project, on one line, 80 characters at most."},` +
	`"goal":{"type":"string","description":"What the project is for."},` +
	`"context":{"type":"string","description":"What the team needs to know to start."},` +
	`"done_looks_like":{"type":"string","description":"What the finished project looks like."},` +
	`"done_when":{"type":"string","description":"One sentence that is true exactly when the project is done."}` +
	`},"required":["name","goal","context","done_looks_like","done_when"]}`

// Proposed is the result of a proposal that was stored: the model is told the
// owner decides.
const Proposed = "Proposed to the owner as a card; nothing is assigned until they confirm. Tell them what you proposed and why in one sentence."

// Proposer is what a proposal is handed to once it is valid; orchestrate
// stores it and tells the bus. It returns an error to refuse one, such as a
// proposal already waiting for the owner.
type Proposer func(ctx context.Context, name string, brief project.Brief) (project.Project, error)

// Definition is propose_project, handing each valid proposal to p. It needs
// the project.propose capability, which the orchestrator grants for a run
// of the Chief of Staff only. It is read-tier: it changes nothing the owner
// has not then confirmed.
// See docs/model.md#5-the-chief-of-staffs-routing-policy.
func Definition(p Proposer) tools.Definition {
	return tools.Definition{
		Name:        "propose_project",
		Description: description,
		Parameters:  json.RawMessage(parameters),
		Capability:  project.CanPropose,
		Tier:        tools.TierRead,
		Execute:     execute(p),
		Describe:    describe,
		Validate:    func(raw json.RawMessage) error { _, _, err := parse(raw); return err },
	}
}

// args is a call's arguments.
type args struct {
	Name          string `json:"name"`
	Goal          string `json:"goal"`
	Context       string `json:"context"`
	DoneLooksLike string `json:"done_looks_like"`
	DoneWhen      string `json:"done_when"`
}

// parse is the name and the brief a call carries, or the reason to refuse
// it. The bounds are project.Validate's: a name of one to 80 runes and four
// parts of one to 4,000 runes each. The name must also be one plain line, as
// the owner reads it in a card and a folder is named from it later. The
// model's text is returned as it wrote it, never changed.
func parse(raw json.RawMessage) (string, project.Brief, error) {
	var a args
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", project.Brief{}, errors.New("the arguments are not a JSON object with a name, goal, context, done_looks_like and done_when")
	}
	brief := project.Brief{Goal: a.Goal, Context: a.Context, DoneLooksLike: a.DoneLooksLike, DoneWhen: a.DoneWhen}
	if memory.OneLine(a.Name) != a.Name {
		return "", project.Brief{}, errors.New("the name holds a line break or another control character; write it on one line as plain text")
	}
	if err := project.Validate(a.Name, brief, ""); err != nil {
		return "", project.Brief{}, err
	}
	return a.Name, brief, nil
}

func execute(p Proposer) func(context.Context, tools.Call) (tools.Result, error) {
	return func(ctx context.Context, call tools.Call) (tools.Result, error) {
		name, brief, err := parse(call.Args)
		if err != nil {
			return tools.Result{}, err
		}
		if _, err := p(ctx, name, brief); err != nil {
			return tools.Result{}, err
		}
		return tools.Result{Text: Proposed}, nil
	}
}

// describe is the call as the owner reads it: "Propose a project: <name>",
// the name cut to 60 characters. It never fails: a call it cannot read, or
// one with no name, reads "Propose a project".
func describe(raw json.RawMessage) string {
	var a args
	_ = json.Unmarshal(raw, &a) // a describer never fails
	name := strings.TrimSpace(memory.OneLine(a.Name))
	if name == "" {
		return "Propose a project"
	}
	if utf8.RuneCountInString(name) > maxDescribed {
		name = string([]rune(name)[:maxDescribed-1]) + "…"
	}
	return "Propose a project: " + name
}
