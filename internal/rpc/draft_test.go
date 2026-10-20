package rpc

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func TestDraftBot_ProposesAndStoresNothing(t *testing.T) {
	r := newRig(t, answer(`{"name": "Wren", "role": "Research Analyst", "job_description": "You read up on things.", `,
		`"working_style": "Direct and brief", "tier": "balanced"}`))
	c := newClients(serve(t, r.opts), testToken)

	got := unary(t, c.bot.DraftBot, &hearthv1.DraftBotRequest{
		Description: "read up on competitors", TemplateId: "writer", Tier: hearthv1.Tier_TIER_FAST,
	})
	if got.GetName() != "Wren" || got.GetRole() != "Research Analyst" || got.GetJobDescription() != "You read up on things." ||
		got.GetWorkingStyle() != "Direct and brief" || got.GetTier() != hearthv1.Tier_TIER_BALANCED {
		t.Errorf("DraftBot = %v", got)
	}
	if bots := unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots(); len(bots) != 0 {
		t.Errorf("a draft stored %d bots", len(bots))
	}
}

func TestDraftBot_WithNoDrafterIsUnavailable(t *testing.T) {
	r := newRig(t)
	r.opts.Drafter = nil
	c := newClients(serve(t, r.opts), testToken)
	_, err := c.bot.DraftBot(t.Context(), connect.NewRequest(&hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST}))
	if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "sign in") {
		t.Errorf("err = %v, want unavailable in plain words", err)
	}
}

func TestDraftBot_ErrorsAreInPlainWords(t *testing.T) {
	t.Run("a reply that cannot be read", func(t *testing.T) {
		r := newRig(t, answer("Sure, I would hire a friendly person."))
		c := newClients(serve(t, r.opts), testToken)
		_, err := c.bot.DraftBot(t.Context(), connect.NewRequest(&hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST}))
		if connect.CodeOf(err) != connect.CodeInternal || !strings.Contains(err.Error(), "could not be read") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("no model", func(t *testing.T) {
		r := newRig(t)
		r.opts.Drafter = drafterOver(func(org.Bot) (agent.Stream, error) { return nil, errors.New("no account is signed in") }, nil)
		c := newClients(serve(t, r.opts), testToken)
		_, err := c.bot.DraftBot(t.Context(), connect.NewRequest(&hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST}))
		if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "no account is signed in") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("nothing to draft from", func(t *testing.T) {
		r := newRig(t)
		c := newClients(serve(t, r.opts), testToken)
		_, err := c.bot.DraftBot(t.Context(), connect.NewRequest(&hearthv1.DraftBotRequest{Tier: hearthv1.Tier_TIER_FAST}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("err = %v", err)
		}
	})
}

// rows is what the store holds that a draft or a check must leave alone: the
// bots, their conversations and the projects.
func rows(t *testing.T, c clients) []proto.Message {
	t.Helper()
	var out []proto.Message
	for _, b := range unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots() {
		out = append(out, b)
		for _, cv := range unary(t, c.conv.ListConversations, &hearthv1.ListConversationsRequest{BotId: b.GetId()}).GetConversations() {
			out = append(out, cv)
		}
	}
	for _, p := range unary(t, c.proj.ListProjects, &hearthv1.ListProjectsRequest{IncludeArchived: true}).GetProjects() {
		out = append(out, p)
	}
	return out
}

func TestDraftsAndChecksChangeNoStoreRow(t *testing.T) {
	r := newRig(t,
		answer(`{"name": "Launch", "goal": "Ship it.", "done_when": "It is better.", "target_date": "2020-01-01"}`),
		answer(`{"name": "Wren", "role": "Chief of Staff", "job_description": "Helps."}`))
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	before := rows(t, c)
	if len(before) == 0 {
		t.Fatal("the rig holds no rows to compare")
	}

	unary(t, c.proj.CheckProject, &hearthv1.CheckProjectRequest{Name: "Launch", TargetDate: "2020-01-01"})
	unary(t, c.bot.CheckBot, &hearthv1.CheckBotRequest{Name: "Wren", Role: "Chief of Staff", JobDescription: "Helps."})
	unary(t, c.proj.DraftProject, &hearthv1.DraftProjectRequest{Description: "ship it"})
	unary(t, c.bot.DraftBot, &hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST})

	after := rows(t, c)
	if len(after) != len(before) {
		t.Fatalf("the store holds %d rows after, %d before", len(after), len(before))
	}
	for i := range before {
		if !proto.Equal(before[i], after[i]) {
			t.Errorf("row %d changed:\n%v\n%v", i, before[i], after[i])
		}
	}
}

func TestDraftProject_ProposesWithTheRuleConcernsAndStoresNothing(t *testing.T) {
	r := newRig(t, answer(`{"name": "Launch", "goal": "Ship it.", "context": "Small.", "done_looks_like": "A page. It works.", `,
		`"done_when": "It is better.", "target_date": "2020-01-01"}`))
	c := newClients(serve(t, r.opts), testToken)
	before := rows(t, c)
	got := unary(t, c.proj.DraftProject, &hearthv1.DraftProjectRequest{Description: "ship it"})
	if got.GetName() != "Launch" || got.GetBrief().GetGoal() != "Ship it." || got.GetTargetDate() != "2020-01-01" {
		t.Errorf("DraftProject = %v", got)
	}
	var fields []string
	for _, cn := range got.GetConcerns() {
		fields = append(fields, cn.GetField())
	}
	if !slices.Equal(fields, []string{"done_when", "target_date"}) {
		t.Errorf("concerns on %v, want the vague done_when and the passed date, in field order", fields)
	}
	if len(rows(t, c)) != len(before) {
		t.Error("a draft changed the store")
	}
}

func TestDraftProject_EmptyDoneWhenIsAMalformedReply(t *testing.T) {
	r := newRig(t, answer(`{"goal": "Ship it.", "done_when": ""}`))
	c := newClients(serve(t, r.opts), testToken)
	_, err := c.proj.DraftProject(t.Context(), connect.NewRequest(&hearthv1.DraftProjectRequest{Description: "ship it"}))
	if connect.CodeOf(err) != connect.CodeInternal || !strings.Contains(err.Error(), "could not be read") {
		t.Errorf("err = %v", err)
	}
}

func TestDraftProject_RequestBoundsAndNoDrafter(t *testing.T) {
	t.Run("no drafter", func(t *testing.T) {
		r := newRig(t)
		r.opts.ProjectDrafter = nil
		c := newClients(serve(t, r.opts), testToken)
		_, err := c.proj.DraftProject(t.Context(), connect.NewRequest(&hearthv1.DraftProjectRequest{Description: "ship it"}))
		if connect.CodeOf(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "sign in") {
			t.Errorf("err = %v, want unavailable in plain words", err)
		}
	})
	t.Run("no description", func(t *testing.T) {
		c := newClients(serve(t, newRig(t).opts), testToken)
		_, err := c.proj.DraftProject(t.Context(), connect.NewRequest(&hearthv1.DraftProjectRequest{Description: " \n"}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("a description over 4,000 characters never reaches the drafter", func(t *testing.T) {
		r := newRig(t)
		called := false
		r.opts.ProjectDrafter = func(context.Context, string) (ProjectDraft, error) { called = true; return ProjectDraft{}, nil }
		c := newClients(serve(t, r.opts), testToken)
		_, err := c.proj.DraftProject(t.Context(), connect.NewRequest(&hearthv1.DraftProjectRequest{Description: strings.Repeat("é", 4001)}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument || called {
			t.Errorf("err = %v, drafter called %v, want invalid_argument and no call", err, called)
		}
		_, err = c.proj.DraftProject(t.Context(), connect.NewRequest(&hearthv1.DraftProjectRequest{Description: strings.Repeat("é", 4000)}))
		if err != nil || !called {
			t.Errorf("4,000 characters: err = %v, called %v, want it through", err, called)
		}
	})
}

func TestCheckProject_AndCheckBot(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})

	got := unary(t, c.proj.CheckProject, &hearthv1.CheckProjectRequest{
		Name: "Launch", Brief: &hearthv1.ProjectBrief{Goal: "g", DoneWhen: "d1"}, TargetDate: "2000-01-01",
	})
	if len(got.GetConcerns()) != 1 || got.GetConcerns()[0].GetField() != "target_date" || got.GetConcerns()[0].GetConcern() != "That date has already passed." {
		t.Errorf("CheckProject = %v, want the past date", got)
	}
	if got := unary(t, c.proj.CheckProject, &hearthv1.CheckProjectRequest{Name: "Launch", Brief: &hearthv1.ProjectBrief{DoneWhen: "d1"}}); len(got.GetConcerns()) != 0 {
		t.Errorf("CheckProject on a sound project = %v, want none", got)
	}

	bot := unary(t, c.bot.CheckBot, &hearthv1.CheckBotRequest{
		Name: "Wren", Role: "chief of staff", JobDescription: "You keep the calendar and the inbox tidy, and tell me what needs me.",
	})
	if len(bot.GetConcerns()) != 1 || bot.GetConcerns()[0].GetField() != "role" ||
		bot.GetConcerns()[0].GetConcern() != "You already have Chief of Staff, your Chief of Staff. A second one is fine if they'll do different work." {
		t.Errorf("CheckBot = %v, want the duplicate role", bot)
	}
}

func TestDraftBot_CarriesTheRuleConcerns(t *testing.T) {
	r := newRig(t, answer(`{"name": "Wren", "role": "Chief of Staff", "job_description": "Helps."}`))
	c := newClients(serve(t, r.opts), testToken)
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	got := unary(t, c.bot.DraftBot, &hearthv1.DraftBotRequest{Description: "help", Tier: hearthv1.Tier_TIER_FAST})
	var fields []string
	for _, cn := range got.GetConcerns() {
		fields = append(fields, cn.GetField())
	}
	if !slices.Equal(fields, []string{"role", "job_description"}) {
		t.Errorf("DraftBot concerns on %v, want the duplicate role and the thin job description", fields)
	}
}
