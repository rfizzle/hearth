package rpc

import (
	"slices"
	"testing"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// Four model concerns and one rule concern serve three, the rule's first; an
// unknown field is dropped and a suggestion with a line break is one line.
func TestDraftProject_ModelConcernsJoinBehindTheRulesAndTheCap(t *testing.T) {
	r := newRig(t, answer(`{"name": "Launch", "goal": "Ship it.", "context": "Small.", "done_looks_like": "A page.", `,
		`"done_when": "It is better.", "target_date": "", "concerns": [`,
		`{"field": "done_when", "concern": "Model on a rule field.", "suggestion": ""}, `,
		`{"field": "tier", "concern": "Unknown field.", "suggestion": ""}, `,
		`{"field": "name", "concern": "Too plain.", "suggestion": "Launch\nthe shop"}, `,
		`{"field": "goal", "concern": "Too short.", "suggestion": ""}, `,
		`{"field": "context", "concern": "Third model one.", "suggestion": ""}]}`))
	c := newClients(serve(t, r.opts), testToken)
	got := unary(t, c.proj.DraftProject, &hearthv1.DraftProjectRequest{Description: "ship it"})
	var fields []string
	for _, cn := range got.GetConcerns() {
		fields = append(fields, cn.GetField())
	}
	if !slices.Equal(fields, []string{"done_when", "name", "goal"}) {
		t.Fatalf("concerns on %v, want the rule's done_when, then the model's name and goal", fields)
	}
	if got.GetConcerns()[1].GetSuggestion() != "Launch the shop" {
		t.Errorf("suggestion = %q, want it on one line", got.GetConcerns()[1].GetSuggestion())
	}
	if got.GetBrief().GetDoneWhen() != "It is better." || got.GetName() != "Launch" {
		t.Errorf("a model concern changed a field: %v", got)
	}
}

func TestDraftBot_ModelConcernsJoinBehindTheRulesAndTheCap(t *testing.T) {
	r := newRig(t, answer(`{"name": "Wren", "role": "Scheduler", "job_description": "Keeps the calendar tidy and books every meeting I ask for.", `,
		`"concerns": [{"field": "name", "concern": "Easy to confuse.", "suggestion": "Wren B."}, `,
		`{"field": "nope", "concern": "Unknown.", "suggestion": ""}, `,
		`{"field": "working_style", "concern": "Missing.", "suggestion": "Brief"}]}`))
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	got := unary(t, c.bot.DraftBot, &hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST})
	var fields []string
	for _, cn := range got.GetConcerns() {
		fields = append(fields, cn.GetField())
	}
	if !slices.Equal(fields, []string{"name", "working_style"}) {
		t.Errorf("DraftBot concerns on %v, want the model's name and working_style", fields)
	}
	if got.GetName() != "Wren" {
		t.Errorf("a model concern changed the name to %q", got.GetName())
	}
}
