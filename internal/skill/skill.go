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

// Templates are the role templates hearth ships, in name order.
func Templates() []org.RoleTemplate {
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
		out = append(out, org.RoleTemplate{
			ID:             s.Name,
			Role:           s.Metadata["role"],
			Summary:        s.Description,
			JobDescription: body,
			SuggestedTier:  tier,
		})
	}
	return out
}
