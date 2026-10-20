package store

import (
	"errors"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func chief(root string) (org.Bot, []org.Grant, org.Conversation) {
	bot, grants, dm := org.NewBot("Chief of Staff", "Chief of Staff", "Routes.", org.TierBalanced, root)
	bot.Scope = org.ScopeOrg
	return bot, grants, dm
}

func TestPutOrgWithCoordinator_HiresOnceWithItsDMAndGrants(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	if _, err := db.Coordinator(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Coordinator before onboarding: %v, want ErrNotFound", err)
	}

	bot, grants, dm := chief(orgFixture.WorkspacePath)
	cos, hired, err := db.PutOrgWithCoordinator(ctx, orgFixture, bot, grants, dm)
	if err != nil || !hired || cos.ID == "" || cos.DMConversationID == "" || cos.Scope != org.ScopeOrg {
		t.Fatalf("PutOrgWithCoordinator = %+v, %v, %v", cos, hired, err)
	}
	if got, err := db.Org(ctx); err != nil || got != orgFixture {
		t.Fatalf("Org = %+v, %v", got, err)
	}
	if got, err := db.Coordinator(ctx); err != nil || got.ID != cos.ID || got.Scope != org.ScopeOrg ||
		got.DMConversationID != cos.DMConversationID {
		t.Fatalf("Coordinator = %+v, %v, want %+v", got, err, cos)
	}
	if c, err := db.Conversation(ctx, cos.DMConversationID); err != nil || c.Kind != org.ConversationDM || c.BotID != cos.ID {
		t.Fatalf("the DM = %+v, %v", c, err)
	}
	if g, err := db.GrantsFor(ctx, cos.ID); err != nil || org.Allowed(g, org.AskOwner) != org.ModeAllowed ||
		org.Allowed(g, org.Remember) != org.ModeAllowed {
		t.Fatalf("grants = %+v, %v, want a new bot's", g, err)
	}

	renamed := org.Org{Name: "Acme Two", WorkspacePath: orgFixture.WorkspacePath}
	bot, grants, dm = chief(orgFixture.WorkspacePath)
	if _, hired, err := db.PutOrgWithCoordinator(ctx, renamed, bot, grants, dm); err != nil || hired {
		t.Fatalf("a second save hired %v, %v, want nobody", hired, err)
	}
	if got, _ := db.Org(ctx); got != renamed {
		t.Fatalf("Org after the second save = %+v, want it renamed", got)
	}
	if bots, err := db.Bots(ctx); err != nil || len(bots) != 1 {
		t.Fatalf("Bots = %+v, %v, want the one Chief of Staff", bots, err)
	}
}

func TestPutOrgWithCoordinator_ATakenFolderSavesTheOrgAndHiresNobody(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	if err := db.PutOrg(ctx, orgFixture); err != nil {
		t.Fatal(err)
	}
	hire(t, db, "chief of staff")
	renamed := org.Org{Name: "Acme Two", WorkspacePath: orgFixture.WorkspacePath}
	bot, grants, dm := chief(orgFixture.WorkspacePath)
	if _, hired, err := db.PutOrgWithCoordinator(ctx, renamed, bot, grants, dm); err != nil || hired {
		t.Fatalf("PutOrgWithCoordinator over a taken folder: %v, %v, want the org saved and nobody hired", hired, err)
	}
	if got, _ := db.Org(ctx); got != renamed {
		t.Fatalf("Org = %+v, want it renamed", got)
	}
	if bots, _ := db.Bots(ctx); len(bots) != 1 || bots[0].Scope != org.ScopeTeam {
		t.Fatalf("Bots = %+v, want only the team bot", bots)
	}
	if _, err := db.Coordinator(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Coordinator = %v, want ErrNotFound", err)
	}
}
