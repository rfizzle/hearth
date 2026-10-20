package rpc

import (
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

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
		r.opts.Drafter = drafterOver(func(org.Tier) (agent.Stream, error) { return nil, errors.New("no account is signed in") }, nil)
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
