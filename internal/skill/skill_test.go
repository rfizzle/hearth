package skill

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func TestTemplates_LoadsTheThreeRoles(t *testing.T) {
	got := Templates()
	want := []string{"planner", "research-analyst", "writer"}
	if len(got) != len(want) {
		t.Fatalf("Templates = %d, want %d", len(got), len(want))
	}
	for i, tpl := range got {
		if tpl.ID != want[i] {
			t.Errorf("template %d id = %q, want %q", i, tpl.ID, want[i])
		}
		if tpl.Role == "" || tpl.Summary == "" || tpl.JobDescription == "" || tpl.SuggestedTier == 0 {
			t.Errorf("template %s has an empty field: %+v", tpl.ID, tpl)
		}
		if tpl.Scope != org.ScopeTeam {
			t.Errorf("template %s is offered for hire at scope %d, want a team's", tpl.ID, tpl.Scope)
		}
	}
}

func TestChiefOfStaff_IsAnOrgLevelBotWithTheRoutingPolicy(t *testing.T) {
	root := filepath.Join("/", "Users", "me", "Hearth")
	bot, grants, dm := ChiefOfStaff(root)
	if bot.Name != "Chief of Staff" || bot.Role != "Chief of Staff" || bot.Tier != org.TierBalanced || bot.Scope != org.ScopeOrg {
		t.Fatalf("bot = %+v, want the Chief of Staff at Balanced and the org level", bot)
	}
	if err := org.CheckName(bot.Name); err != nil || bot.FolderPath != filepath.Join(root, "Chief of Staff") {
		t.Fatalf("folder %q, %v, want one under the Hearth folder that is not the Inbox", bot.FolderPath, err)
	}
	if org.Allowed(grants, org.AskOwner) != org.ModeAllowed || org.Allowed(grants, org.Remember) != org.ModeAllowed {
		t.Errorf("grants = %+v, want a new bot's", grants)
	}
	if dm.Kind != org.ConversationDM || dm.Title != bot.Name {
		t.Errorf("dm = %+v", dm)
	}

	// The five rules of the routing policy, in order.
	rules := []string{
		"1. Answer directly when the request is small",
		"2. Otherwise find the one team whose purpose matches",
		"3. Split the work across several teams only when",
		"4. When the request is ambiguous",
		"5. Never restate work a team already did",
	}
	at := 0
	for _, r := range rules {
		i := strings.Index(bot.JobDescription[at:], r)
		if i < 0 {
			t.Fatalf("the job description lacks %q in order:\n%s", r, bot.JobDescription)
		}
		at += i
	}
	for _, tool := range []string{"assign", "ask_owner", "remember", "files.", "web.", "propose"} {
		if strings.Contains(strings.ToLower(bot.JobDescription), tool) {
			t.Errorf("the job description names the tool %q", tool)
		}
	}
}

// The introduction the app shows lives in the template's frontmatter: a
// purpose, three lines on how it works and three things to try.
func TestChiefOfStaff_CarriesItsIntroduction(t *testing.T) {
	data, err := builtin.ReadFile("builtin/" + chiefOfStaffID + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	front := strings.SplitN(string(data), "\n---\n", 2)[0]
	_, intro, ok := strings.Cut(front, "\nintroduction:\n")
	if !ok {
		t.Fatalf("no introduction in the frontmatter:\n%s", front)
	}
	parts := map[string]string{}
	for _, line := range strings.Split(intro, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), ": "); ok {
			parts[k] = strings.Trim(v, `"`)
		}
	}
	if parts["purpose"] == "" {
		t.Error("the introduction has no purpose")
	}
	for _, k := range []string{"how", "try"} {
		if n := len(strings.Split(parts[k], `\n`)); n != 3 {
			t.Errorf("the introduction's %s has %d lines, want 3", k, n)
		}
	}
}
