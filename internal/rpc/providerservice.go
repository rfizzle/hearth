package rpc

import (
	"context"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func (s *providerService) ListAccounts(_ context.Context, _ *connect.Request[hearthv1.ListAccountsRequest]) (*connect.Response[hearthv1.ListAccountsResponse], error) {
	return reply(&hearthv1.ListAccountsResponse{Accounts: each(s.opts.Accounts.List(), accountToProto)}, nil)
}

// SetAPIKey keeps a key. The base URL belongs to the OpenAI-compatible
// provider, which hearth cannot use yet, so it is not read.
func (s *providerService) SetAPIKey(_ context.Context, req *connect.Request[hearthv1.SetAPIKeyRequest]) (*connect.Response[hearthv1.SetAPIKeyResponse], error) {
	if err := need("key", req.Msg.GetKey()); err != nil {
		return nil, err
	}
	p, err := fromProto(providerNames, req.Msg.GetProvider())
	if err != nil {
		return nil, err
	}
	if err := s.opts.Accounts.SetKey(p, req.Msg.GetKey()); err != nil {
		// The provider words, for the person, why it cannot take this key.
		return nil, invalid("%v", err)
	}
	return reply(&hearthv1.SetAPIKeyResponse{Account: s.account(p)}, nil)
}

func (s *providerService) ClearCredential(ctx context.Context, req *connect.Request[hearthv1.ClearCredentialRequest]) (*connect.Response[hearthv1.ClearCredentialResponse], error) {
	p, err := fromProto(providerNames, req.Msg.GetProvider())
	if err != nil {
		return nil, err
	}
	return reply(&hearthv1.ClearCredentialResponse{}, s.opts.Accounts.Clear(ctx, p))
}

// StartChatGPTLogin returns the page to sign in on at once; the sign-in
// finishes in the background and the account change reaches the bus. The
// app opens the page, so the browser opens from the process with a window.
// See docs/architecture.md#3-the-contract-sketch
func (s *providerService) StartChatGPTLogin(ctx context.Context, _ *connect.Request[hearthv1.StartChatGPTLoginRequest]) (*connect.Response[hearthv1.StartChatGPTLoginResponse], error) {
	page, err := s.opts.Accounts.StartLogin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&hearthv1.StartChatGPTLoginResponse{AuthorizeUrl: page}), nil
}

// account is p's account as List reports it now.
func (s *providerService) account(p provider.Provider) *hearthv1.ProviderAccount {
	for _, a := range s.opts.Accounts.List() {
		if a.Provider == p {
			return accountToProto(a)
		}
	}
	return nil
}
