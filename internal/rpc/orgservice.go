package rpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/workspace"
)

func (s *orgService) GetOrg(ctx context.Context, _ *connect.Request[hearthv1.GetOrgRequest]) (*connect.Response[hearthv1.GetOrgResponse], error) {
	o, err := s.opts.Store.Org(ctx)
	return reply(&hearthv1.GetOrgResponse{Org: orgToProto(o)}, err)
}

// UpdateOrg creates the Hearth folder, then keeps the org: a folder that
// cannot be made is refused before the org points at it.
func (s *orgService) UpdateOrg(ctx context.Context, req *connect.Request[hearthv1.UpdateOrgRequest]) (*connect.Response[hearthv1.UpdateOrgResponse], error) {
	m := req.Msg
	if err := need("name", m.GetName(), "workspace_path", m.GetWorkspacePath()); err != nil {
		return nil, err
	}
	if err := workspace.Ensure(m.GetWorkspacePath()); err != nil {
		return nil, invalid("%v", err)
	}
	o := org.Org{Name: m.GetName(), WorkspacePath: m.GetWorkspacePath()}
	return reply(&hearthv1.UpdateOrgResponse{Org: orgToProto(o)}, s.opts.Store.PutOrg(ctx, o))
}
