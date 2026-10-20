package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// GetOrg leaves the org unset before the owner has named it: nothing is
// missing, onboarding has not happened.
func (s *orgService) GetOrg(ctx context.Context, _ *connect.Request[hearthv1.GetOrgRequest]) (*connect.Response[hearthv1.GetOrgResponse], error) {
	o, found, err := s.opts.Store.FindOrg(ctx)
	if err != nil {
		return nil, fail(err)
	}
	res := &hearthv1.GetOrgResponse{}
	if found {
		res.Org = orgToProto(o)
	}
	return connect.NewResponse(res), nil
}

// UpdateOrg creates the Hearth folder, then keeps the org: a folder that
// cannot be made is refused before the org points at it.
func (s *orgService) UpdateOrg(ctx context.Context, req *connect.Request[hearthv1.UpdateOrgRequest]) (*connect.Response[hearthv1.UpdateOrgResponse], error) {
	m := req.Msg
	if err := need("name", m.GetName(), "workspace_path", m.GetWorkspacePath()); err != nil {
		return nil, err
	}
	if s.opts.EnsureWorkspace == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no way to create the Hearth folder is set"))
	}
	if err := s.opts.EnsureWorkspace(m.GetWorkspacePath()); err != nil {
		return nil, invalid("%v", err)
	}
	o := org.Org{Name: m.GetName(), WorkspacePath: m.GetWorkspacePath()}
	return reply(&hearthv1.UpdateOrgResponse{Org: orgToProto(o)}, s.opts.Store.PutOrg(ctx, o))
}
