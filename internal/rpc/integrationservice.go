package rpc

import (
	"context"

	"connectrpc.com/connect"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func (s *integrationService) ListIntegrations(_ context.Context, _ *connect.Request[hearthv1.ListIntegrationsRequest]) (*connect.Response[hearthv1.ListIntegrationsResponse], error) {
	return reply(&hearthv1.ListIntegrationsResponse{Integrations: each(s.opts.Integrations.List(), integrationAccountToProto)}, nil)
}

func (s *integrationService) SetIntegrationKey(_ context.Context, req *connect.Request[hearthv1.SetIntegrationKeyRequest]) (*connect.Response[hearthv1.SetIntegrationKeyResponse], error) {
	name, err := fromProto(integrationNames, req.Msg.GetIntegration())
	if err != nil {
		return nil, err
	}
	a, err := s.opts.Integrations.SetKey(name, req.Msg.GetKey())
	return reply(&hearthv1.SetIntegrationKeyResponse{Integration: integrationAccountToProto(a)}, err)
}

func (s *integrationService) ClearIntegration(_ context.Context, req *connect.Request[hearthv1.ClearIntegrationRequest]) (*connect.Response[hearthv1.ClearIntegrationResponse], error) {
	name, err := fromProto(integrationNames, req.Msg.GetIntegration())
	if err != nil {
		return nil, err
	}
	_, err = s.opts.Integrations.Clear(name)
	return reply(&hearthv1.ClearIntegrationResponse{}, err)
}
