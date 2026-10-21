package skill

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/intro"
)

func TestTemplates_LoadsTheFiveRoles(t *testing.T) {
	got := Templates()
	want := []string{"finance-analyst", "marketing-lead", "planner", "research-analyst", "writer"}
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

// Every shipped template carries an introduction that passes the check, and a
// bot hired from the Chief of Staff's template opens with the template's.
func TestTemplates_CarryTheirIntroductions(t *testing.T) {
	for _, tpl := range all() {
		if err := intro.Validate(tpl.Introduction); err != nil {
			t.Errorf("template %s: %v", tpl.ID, err)
		}
	}
	bot, _, _ := ChiefOfStaff(filepath.Join("/", "Users", "me", "Hearth"))
	if !strings.HasPrefix(bot.Introduction.Purpose, "I run your office.") ||
		bot.Introduction.How[1].Symbol != "arrow.triangle.branch" ||
		bot.Introduction.Try[0] != "Summarise this week's customer calls" {
		t.Errorf("the Chief of Staff's introduction = %+v, want the template's", bot.Introduction)
	}
}

func TestIntroductionOf_RefusesWhatIsNotAnIntroduction(t *testing.T) {
	for name, file := range map[string]string{
		"none":        "---\nname: x\n---\nbody",
		"unquoted":    "---\nintroduction:\n  purpose: plain\n---\n",
		"two prompts": "---\nintroduction:\n  purpose: \"p\"\n  how: \"a x\\nb y\\nc z\"\n  try: \"one\\ntwo\"\n---\n",
		"no symbol":   "---\nintroduction:\n  purpose: \"p\"\n  how: \"X y\\nb y\\nc z\"\n  try: \"1\\n2\\n3\"\n---\n",
	} {
		if _, err := introductionOf(file); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
