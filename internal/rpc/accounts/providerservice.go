package accounts

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

func (s *providerService) ListAccounts(_ context.Context, _ *connect.Request[hearthv1.ListAccountsRequest]) (*connect.Response[hearthv1.ListAccountsResponse], error) {
	return wire.Reply(&hearthv1.ListAccountsResponse{Accounts: wire.Each(s.opts.Accounts.List(), accountToProto)}, nil)
}

// SetAPIKey keeps a key, or for an OpenAI-compatible service the key and
// the service's settings as a profile. The account it returns carries the
// key's last four characters, never the key.
// See docs/model.md#providers-and-models
func (s *providerService) SetAPIKey(ctx context.Context, req *connect.Request[hearthv1.SetAPIKeyRequest]) (*connect.Response[hearthv1.SetAPIKeyResponse], error) {
	m := req.Msg
	p, err := wire.FromProto(wire.ProviderNames, m.GetProvider())
	if err != nil {
		return nil, err
	}
	if p == provider.Compatible {
		acct, err := s.opts.Accounts.SaveProfile(ctx, provider.Profile{ID: m.GetProfile(), Name: m.GetDisplayName(), BaseURL: m.GetBaseUrl(), DefaultModel: m.GetDefaultModel()}, m.GetKey())
		if err != nil && !errors.Is(err, provider.ErrNoProfile) {
			// The provider words, for the person, what to fix.
			err = wire.Invalid("%v", err)
		}
		return wire.Reply(&hearthv1.SetAPIKeyResponse{Account: accountToProto(acct)}, err)
	}
	if err := wire.Need("key", m.GetKey()); err != nil {
		return nil, err
	}
	if err := s.opts.Accounts.SetKey(p, m.GetKey()); err != nil {
		// The provider words, for the person, why it cannot take this key.
		return nil, wire.Invalid("%v", err)
	}
	return wire.Reply(&hearthv1.SetAPIKeyResponse{Account: s.account(p)}, nil)
}

func (s *providerService) ClearCredential(ctx context.Context, req *connect.Request[hearthv1.ClearCredentialRequest]) (*connect.Response[hearthv1.ClearCredentialResponse], error) {
	p, err := wire.FromProto(wire.ProviderNames, req.Msg.GetProvider())
	if err != nil {
		return nil, err
	}
	if p == provider.Compatible {
		if err := wire.Need("profile", req.Msg.GetProfile()); err != nil {
			return nil, err
		}
		return wire.Reply(&hearthv1.ClearCredentialResponse{}, s.opts.Accounts.ClearProfile(req.Msg.GetProfile()))
	}
	return wire.Reply(&hearthv1.ClearCredentialResponse{}, s.opts.Accounts.Clear(ctx, p))
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

// SyncCredentials takes the records the app keeps in its Keychain and
// returns the ones the core held in files that the app lacked. The request
// carries values in and the reply carries values out (records_to_store); no
// error text and no event does.
// See docs/architecture.md#2-responsibilities
func (s *providerService) SyncCredentials(_ context.Context, req *connect.Request[hearthv1.SyncCredentialsRequest]) (*connect.Response[hearthv1.SyncCredentialsResponse], error) {
	out, err := s.opts.Accounts.Sync(wire.CredentialsFromProto(req.Msg.GetRecords()))
	if err != nil {
		return nil, wire.Invalid("%v", err)
	}
	return connect.NewResponse(&hearthv1.SyncCredentialsResponse{RecordsToStore: wire.Each(out, wire.CredentialToProto)}), nil
}

// ExportCredential is the one reply that carries a credential value outside
// a sync: the record under one name, for the app to store after a
// credential_changed event.
func (s *providerService) ExportCredential(_ context.Context, req *connect.Request[hearthv1.ExportCredentialRequest]) (*connect.Response[hearthv1.ExportCredentialResponse], error) {
	if err := wire.Need("name", req.Msg.GetName()); err != nil {
		return nil, err
	}
	rec, err := s.opts.Accounts.Export(req.Msg.GetName())
	if err != nil {
		return nil, wire.Fail(err)
	}
	return connect.NewResponse(&hearthv1.ExportCredentialResponse{Record: wire.CredentialToProto(rec)}), nil
}
