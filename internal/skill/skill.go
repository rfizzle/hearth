package skill

import (
	"embed"
	"fmt"

	agentskill "github.com/rfizzle/agentcore/skill"

	"github.com/rfizzle/hearth/internal/org"
)

//go:embed builtin/*/SKILL.md
var builtin embed.FS

// tiers maps the tier a template's metadata names.
var tiers = map[string]org.Tier{"fast": org.TierFast, "balanced": org.TierBalanced, "best": org.TierBest}

// chiefOfStaffID is the id of the template the Chief of Staff is hired from.
const chiefOfStaffID = "chief-of-staff"

// Templates are the role templates the owner hires a team member from, in
// name order. An org-level template is not among them: the org's one is
// hired for the owner at onboarding.
// See docs/model.md#people
func Templates() []org.RoleTemplate {
	var out []org.RoleTemplate
	for _, t := range all() {
		if t.Scope == org.ScopeTeam {
			out = append(out, t)
		}
	}
	return out
}

// ChiefOfStaff is the Chief of Staff as a new bot from its shipped template,
// named for its role, with its grants and its DM: an org-level bot whose
// folder is under workspaceRoot like any bot's.
// See docs/model.md#5-the-chief-of-staffs-routing-policy
func ChiefOfStaff(workspaceRoot string) (org.Bot, []org.Grant, org.Conversation) {
	for _, t := range all() {
		if t.ID == chiefOfStaffID {
			bot, grants, dm := org.NewBot(t.Role, t.Role, t.JobDescription, t.SuggestedTier, workspaceRoot)
			bot.Scope = t.Scope
			return bot, grants, dm
		}
	}
	panic("skill: no " + chiefOfStaffID + " template")
}

// all is every shipped template, in name order.
func all() []org.RoleTemplate {
	var out []org.RoleTemplate
	for _, s := range agentskill.Builtins(builtin) {
		body, err := s.Body()
		if err != nil {
			panic(fmt.Sprintf("skill: template %s: %v", s.Name, err))
		}
		tier, ok := tiers[s.Metadata["tier"]]
		if !ok {
			panic(fmt.Sprintf("skill: template %s names tier %q", s.Name, s.Metadata["tier"]))
		}
		scope, ok := scopeOf(s.Metadata["scope"])
		if !ok {
			panic(fmt.Sprintf("skill: template %s names scope %q", s.Name, s.Metadata["scope"]))
		}
		out = append(out, org.RoleTemplate{
			ID:             s.Name,
			Role:           s.Metadata["role"],
			Summary:        s.Description,
			JobDescription: body,
			SuggestedTier:  tier,
			Scope:          scope,
		})
	}
	return out
}

// scopeOf is the scope a template's metadata names; none is a team's.
func scopeOf(name string) (org.Scope, bool) {
	switch name {
	case "", "team":
		return org.ScopeTeam, true
	case "org":
		return org.ScopeOrg, true
	}
	return 0, false
}
