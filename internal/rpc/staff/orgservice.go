package staff

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/skill"
)

// GetOrg leaves the org unset before the owner has named it: nothing is
// missing, onboarding has not happened.
func (s *orgService) GetOrg(ctx context.Context, _ *connect.Request[hearthv1.GetOrgRequest]) (*connect.Response[hearthv1.GetOrgResponse], error) {
	o, found, err := s.opts.Store.FindOrg(ctx)
	if err != nil {
		return nil, wire.Fail(err)
	}
	res := &hearthv1.GetOrgResponse{}
	if found {
		res.Org = orgToProto(o)
	}
	return connect.NewResponse(res), nil
}

// UpdateOrg creates the Hearth folder, then keeps the org: a folder that
// cannot be made is refused before the org points at it. The first save
// hires the Chief of Staff with it, so someone is at the desk when
// onboarding ends; a later save hires nobody.
// See docs/model.md#5-the-chief-of-staffs-routing-policy
func (s *orgService) UpdateOrg(ctx context.Context, req *connect.Request[hearthv1.UpdateOrgRequest]) (*connect.Response[hearthv1.UpdateOrgResponse], error) {
	m := req.Msg
	if err := wire.Need("name", m.GetName(), "workspace_path", m.GetWorkspacePath()); err != nil {
		return nil, err
	}
	if s.opts.EnsureWorkspace == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no way to create the Hearth folder is set"))
	}
	if err := s.opts.EnsureWorkspace(m.GetWorkspacePath()); err != nil {
		return nil, wire.Invalid("%v", err)
	}
	o := org.Org{Name: m.GetName(), WorkspacePath: m.GetWorkspacePath()}
	bot, grants, dm := skill.ChiefOfStaff(o.WorkspacePath)
	bot, hired, err := s.opts.Store.PutOrgWithCoordinator(ctx, o, bot, grants, dm)
	if hired {
		s.opts.Publish(wire.BotChanged{Bot: bot})
	}
	return wire.Reply(&hearthv1.UpdateOrgResponse{Org: orgToProto(o)}, err)
}
