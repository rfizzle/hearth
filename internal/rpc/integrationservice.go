package rpc

import (
	"context"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

func (s *integrationService) ListIntegrations(_ context.Context, _ *connect.Request[hearthv1.ListIntegrationsRequest]) (*connect.Response[hearthv1.ListIntegrationsResponse], error) {
	return wire.Reply(&hearthv1.ListIntegrationsResponse{Integrations: wire.Each(s.opts.Integrations.List(), integrationAccountToProto)}, nil)
}

func (s *integrationService) SetIntegrationKey(_ context.Context, req *connect.Request[hearthv1.SetIntegrationKeyRequest]) (*connect.Response[hearthv1.SetIntegrationKeyResponse], error) {
	name, err := wire.FromProto(integrationNames, req.Msg.GetIntegration())
	if err != nil {
		return nil, err
	}
	a, err := s.opts.Integrations.SetKey(name, req.Msg.GetKey())
	return wire.Reply(&hearthv1.SetIntegrationKeyResponse{Integration: integrationAccountToProto(a)}, err)
}

func (s *integrationService) ClearIntegration(_ context.Context, req *connect.Request[hearthv1.ClearIntegrationRequest]) (*connect.Response[hearthv1.ClearIntegrationResponse], error) {
	name, err := wire.FromProto(integrationNames, req.Msg.GetIntegration())
	if err != nil {
		return nil, err
	}
	_, err = s.opts.Integrations.Clear(name)
	return wire.Reply(&hearthv1.ClearIntegrationResponse{}, err)
}
