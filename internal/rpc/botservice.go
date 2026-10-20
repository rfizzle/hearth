package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
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
	if err := org.CheckName(m.GetName()); err != nil {
		return nil, fail(err)
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
	if s.opts.Drafter == nil {
		return nil, fail(Unavailable(errors.New("no model is set up; sign in to one in Settings")))
	}
	d, err := s.opts.Drafter(ctx, DraftRequest{Description: m.GetDescription(), TemplateID: m.GetTemplateId(), Tier: tier})
	return reply(&hearthv1.DraftBotResponse{
		Name: d.Name, Role: d.Role, JobDescription: d.JobDescription,
		WorkingStyle: d.WorkingStyle, Tier: hearthv1.Tier(d.Tier),
	}, err)
}

// ListGrants is what the bot may do, one row per capability and per folder.
// See docs/model.md#access
func (s *botService) ListGrants(ctx context.Context, req *connect.Request[hearthv1.ListGrantsRequest]) (*connect.Response[hearthv1.ListGrantsResponse], error) {
	id := req.Msg.GetBotId()
	if err := need("bot_id", id); err != nil {
		return nil, err
	}
	bot, err := s.opts.Store.Bot(ctx, id)
	if err != nil {
		return nil, fail(err)
	}
	grants, err := s.opts.Store.GrantsFor(ctx, id)
	return reply(&hearthv1.ListGrantsResponse{Grants: grantsToProto(grants, bot.FolderPath)}, err)
}

// SetGrants replaces the bot's grants with the owner's rows. The inbox
// package refuses a mode a capability never has and the store keeps the
// bot's own folder; the handler widens nothing and answers with the grants
// the store holds, which it tells the bus.
// See docs/model.md#access
func (s *botService) SetGrants(ctx context.Context, req *connect.Request[hearthv1.SetGrantsRequest]) (*connect.Response[hearthv1.SetGrantsResponse], error) {
	m := req.Msg
	if err := need("bot_id", m.GetBotId()); err != nil {
		return nil, err
	}
	grants := make([]org.Grant, 0, len(m.GetGrants()))
	for _, g := range m.GetGrants() {
		one, err := grantFromProto(g)
		if err != nil {
			return nil, err
		}
		grants = append(grants, one)
	}
	if err := inbox.CheckGrants(grants, s.opts.ReservedFolders...); err != nil {
		return nil, fail(err)
	}
	if err := s.opts.Store.ReplaceGrants(ctx, m.GetBotId(), grants); err != nil {
		return nil, fail(err)
	}
	stored, err := s.opts.Store.GrantsFor(ctx, m.GetBotId())
	if err != nil {
		return nil, fail(err)
	}
	s.opts.publish(orchestrate.GrantsChanged{BotID: m.GetBotId(), Grants: stored})
	return connect.NewResponse(&hearthv1.SetGrantsResponse{Grants: s.opts.grantsOf(ctx, m.GetBotId(), stored)}), nil
}
