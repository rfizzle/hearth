package staff

import (
	"context"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

func (s *botService) ListTemplates(_ context.Context, _ *connect.Request[hearthv1.ListTemplatesRequest]) (*connect.Response[hearthv1.ListTemplatesResponse], error) {
	return connect.NewResponse(&hearthv1.ListTemplatesResponse{Templates: wire.Each(s.opts.Templates, templateToProto)}), nil
}

// CreateBot hires the bot into the Hearth folder the org names, then tells
// the bus. The store refuses a folder another bot has.
// See docs/model.md#access
func (s *botService) CreateBot(ctx context.Context, req *connect.Request[hearthv1.CreateBotRequest]) (*connect.Response[hearthv1.CreateBotResponse], error) {
	m := req.Msg
	tier, err := wire.TierFromProto(m.GetTier())
	if err != nil {
		return nil, err
	}
	if err := wire.Need("name", m.GetName(), "job_description", m.GetJobDescription()); err != nil {
		return nil, err
	}
	if err := org.CheckName(m.GetName()); err != nil {
		return nil, wire.Fail(err)
	}
	o, err := s.opts.Store.Org(ctx)
	if err != nil {
		return nil, wire.Fail(err)
	}
	bot, grants, dm := org.NewBot(m.GetName(), m.GetRole(), m.GetJobDescription(), tier, o.WorkspacePath)
	bot.Avatar = m.GetAvatar()
	for _, t := range s.opts.Templates {
		if t.ID == m.GetTemplateId() {
			bot.Introduction = t.Introduction
		}
	}
	if bot, err = s.opts.Store.CreateBot(ctx, bot, grants, dm); err == nil {
		s.opts.Publish(wire.BotChanged{Bot: bot})
	}
	return wire.Reply(&hearthv1.CreateBotResponse{Bot: wire.BotToProto(bot)}, err)
}

func (s *botService) ListBots(ctx context.Context, _ *connect.Request[hearthv1.ListBotsRequest]) (*connect.Response[hearthv1.ListBotsResponse], error) {
	bots, err := s.opts.Store.Bots(ctx)
	if err != nil {
		return nil, wire.Fail(err)
	}
	return connect.NewResponse(&hearthv1.ListBotsResponse{Bots: wire.Each(bots, wire.BotToProto)}), nil
}

func (s *botService) GetBot(ctx context.Context, req *connect.Request[hearthv1.GetBotRequest]) (*connect.Response[hearthv1.GetBotResponse], error) {
	id := req.Msg.GetId()
	if err := wire.Need("id", id); err != nil {
		return nil, err
	}
	bot, err := s.opts.Store.Bot(ctx, id)
	return wire.Reply(&hearthv1.GetBotResponse{Bot: wire.BotToProto(bot)}, err)
}

// DraftBot asks the model, at the Fast tier, to propose a bot from the
// owner's words. It stores nothing: the app shows the proposal and sends
// CreateBot with whatever the owner settles on.
// See docs/model.md#people
func (s *botService) DraftBot(ctx context.Context, req *connect.Request[hearthv1.DraftBotRequest]) (*connect.Response[hearthv1.DraftBotResponse], error) {
	m := req.Msg
	if err := askable(m.GetDescription(), 0, s.opts.Drafter != nil); err != nil {
		return nil, err
	}
	tier, err := wire.TierFromProto(m.GetTier())
	if err != nil {
		return nil, err
	}
	d, err := s.opts.Drafter(ctx, DraftRequest{Description: m.GetDescription(), TemplateID: m.GetTemplateId(), Tier: tier})
	var concerns []concern.Concern
	if err == nil {
		concerns, err = s.check(ctx, d.Name, d.Role, d.JobDescription, d.Concerns)
	}
	return wire.Reply(&hearthv1.DraftBotResponse{
		Name: d.Name, Role: d.Role, JobDescription: d.JobDescription,
		WorkingStyle: d.WorkingStyle, Tier: hearthv1.Tier(d.Tier),
		Concerns: wire.Each(concerns, wire.ConcernToProto),
	}, err)
}

// CheckBot says what looks weak about a hire, by rule and with no model. It
// reads the bots and stores nothing.
// See docs/model.md#owner-interaction
func (s *botService) CheckBot(ctx context.Context, req *connect.Request[hearthv1.CheckBotRequest]) (*connect.Response[hearthv1.CheckBotResponse], error) {
	m := req.Msg
	concerns, err := s.check(ctx, m.GetName(), m.GetRole(), m.GetJobDescription(), nil)
	return wire.Reply(&hearthv1.CheckBotResponse{Concerns: wire.Each(concerns, wire.ConcernToProto)}, err)
}

// check is the concerns for a hire: the rules' against the bots the org has,
// then the model's, if any, behind the same cap.
func (s *botService) check(ctx context.Context, name, role, jobDescription string, model []concern.Concern) ([]concern.Concern, error) {
	bots, err := s.opts.Store.Bots(ctx)
	return concern.Cap(concern.Bot(name, role, jobDescription, bots), model), err
}

// ListGrants is what the bot may do, one row per capability and per folder.
// See docs/model.md#access
func (s *botService) ListGrants(ctx context.Context, req *connect.Request[hearthv1.ListGrantsRequest]) (*connect.Response[hearthv1.ListGrantsResponse], error) {
	id := req.Msg.GetBotId()
	if err := wire.Need("bot_id", id); err != nil {
		return nil, err
	}
	bot, err := s.opts.Store.Bot(ctx, id)
	if err != nil {
		return nil, wire.Fail(err)
	}
	grants, err := s.opts.Store.GrantsFor(ctx, id)
	return wire.Reply(&hearthv1.ListGrantsResponse{Grants: grantsToProto(grants, bot.FolderPath)}, err)
}

// SetGrants replaces the bot's grants with the owner's rows. The inbox
// package refuses a mode a capability never has and the store keeps the
// bot's own folder; the handler widens nothing and answers with the grants
// the store holds, which it tells the bus.
// See docs/model.md#access
func (s *botService) SetGrants(ctx context.Context, req *connect.Request[hearthv1.SetGrantsRequest]) (*connect.Response[hearthv1.SetGrantsResponse], error) {
	m := req.Msg
	if err := wire.Need("bot_id", m.GetBotId()); err != nil {
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
		return nil, wire.Fail(err)
	}
	if err := s.opts.Store.ReplaceGrants(ctx, m.GetBotId(), grants); err != nil {
		return nil, wire.Fail(err)
	}
	stored, err := s.opts.Store.GrantsFor(ctx, m.GetBotId())
	if err != nil {
		return nil, wire.Fail(err)
	}
	s.opts.Publish(orchestrate.GrantsChanged{BotID: m.GetBotId(), Grants: stored})
	return connect.NewResponse(&hearthv1.SetGrantsResponse{Grants: s.opts.grantsOf(ctx, m.GetBotId(), stored)}), nil
}
