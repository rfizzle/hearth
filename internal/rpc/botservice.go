package rpc

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func (s *botService) ListTemplates(_ context.Context, _ *connect.Request[hearthv1.ListTemplatesRequest]) (*connect.Response[hearthv1.ListTemplatesResponse], error) {
	return connect.NewResponse(&hearthv1.ListTemplatesResponse{Templates: each(s.opts.Templates, templateToProto)}), nil
}

// CreateBot hires the bot into the Hearth folder the org names, then tells
// the bus. The store refuses a folder another bot has.
// See docs/model.md#access
func (s *botService) CreateBot(ctx context.Context, req *connect.Request[hearthv1.CreateBotRequest]) (*connect.Response[hearthv1.CreateBotResponse], error) {
	m := req.Msg
	tier, err := tierFromProto(m.GetTier())
	if err != nil {
		return nil, err
	}
	if err := need("name", m.GetName(), "job_description", m.GetJobDescription()); err != nil {
		return nil, err
	}
	o, err := s.opts.Store.Org(ctx)
	if err != nil {
		return nil, fail(err)
	}
	bot, grants, dm := org.NewBot(m.GetName(), m.GetRole(), m.GetJobDescription(), tier, o.WorkspacePath)
	bot.Avatar = m.GetAvatar()
	if bot, err = s.opts.Store.CreateBot(ctx, bot, grants, dm); err == nil {
		s.opts.publish(BotChanged{Bot: bot})
	}
	return reply(&hearthv1.CreateBotResponse{Bot: botToProto(bot)}, err)
}

func (s *botService) ListBots(ctx context.Context, _ *connect.Request[hearthv1.ListBotsRequest]) (*connect.Response[hearthv1.ListBotsResponse], error) {
	bots, err := s.opts.Store.Bots(ctx)
	if err != nil {
		return nil, fail(err)
	}
	return connect.NewResponse(&hearthv1.ListBotsResponse{Bots: each(bots, botToProto)}), nil
}

func (s *botService) GetBot(ctx context.Context, req *connect.Request[hearthv1.GetBotRequest]) (*connect.Response[hearthv1.GetBotResponse], error) {
	id := req.Msg.GetId()
	if err := need("id", id); err != nil {
		return nil, err
	}
	bot, err := s.opts.Store.Bot(ctx, id)
	return reply(&hearthv1.GetBotResponse{Bot: botToProto(bot)}, err)
}

// DraftBot asks the model, at the Fast tier, to propose a bot from the
// owner's words. It stores nothing: the app shows the proposal and sends
// CreateBot with whatever the owner settles on.
// See docs/model.md#people
func (s *botService) DraftBot(ctx context.Context, req *connect.Request[hearthv1.DraftBotRequest]) (*connect.Response[hearthv1.DraftBotResponse], error) {
	m := req.Msg
	if err := need("description", m.GetDescription()); err != nil {
		return nil, err
	}
	tier, err := tierFromProto(m.GetTier())
	if err != nil {
		return nil, err
	}
	in := draft.Input{
		Description: m.GetDescription(), TemplateID: m.GetTemplateId(), Tier: tier,
		Templates: s.opts.Templates, Scrub: s.opts.Scrub,
	}
	if s.opts.Streams != nil {
		if in.Stream, err = s.opts.Streams(org.TierFast); err != nil {
			return nil, fail(fmt.Errorf("%w: %s", draft.ErrUnavailable, agent.Explain(err)))
		}
	}
	d, err := draft.Draft(ctx, in)
	return reply(&hearthv1.DraftBotResponse{
		Name: d.Name, Role: d.Role, JobDescription: d.JobDescription,
		WorkingStyle: d.WorkingStyle, Tier: hearthv1.Tier(d.Tier),
	}, err)
}
