package rpc

import (
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

func TestProviderProfile_SetListClearWithoutTheKey(t *testing.T) {
	const key = "gw-0123456789-secret-wxyz"
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	f := follow(t, c, r.bus)
	compat := hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE

	set := unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{
		Provider: compat, Key: key,
		DisplayName: "Acme gateway", BaseUrl: "https://llm.acme.example/v1", DefaultModel: "acme-large",
	})
	got := set.GetAccount()
	if got.GetProvider() != compat || got.GetProfile() == "" || got.GetState() != hearthv1.AccountState_ACCOUNT_STATE_SIGNED_IN ||
		got.GetDisplayName() != "Acme gateway" || got.GetBaseUrl() != "https://llm.acme.example/v1" || got.GetDefaultModel() != "acme-large" || got.GetLabel() != "wxyz" {
		t.Fatalf("SetAPIKey account = %v", got)
	}
	profile := got.GetProfile()
	added := f.until(func(m *hearthv1.SubscribeResponse) bool {
		return m.GetAccountChanged().GetAccount().GetProfile() == profile
	})

	list := unary(t, c.prov.ListAccounts, &hearthv1.ListAccountsRequest{})
	accounts := list.GetAccounts()
	if len(accounts) != 5 || accounts[4].GetProfile() != profile || accounts[4].GetDisplayName() != "Acme gateway" {
		t.Fatalf("ListAccounts = %v", list)
	}

	// Changing the model with no key keeps the key and the id.
	changed := unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{
		Provider: compat, Profile: profile,
		DisplayName: "Acme gateway", BaseUrl: "https://llm.acme.example/v1", DefaultModel: "acme-small",
	})
	if a := changed.GetAccount(); a.GetProfile() != profile || a.GetDefaultModel() != "acme-small" || a.GetLabel() != "wxyz" {
		t.Errorf("changed account = %v", a)
	}

	unary(t, c.prov.ClearCredential, &hearthv1.ClearCredentialRequest{Provider: compat, Profile: profile})
	cleared := f.until(func(m *hearthv1.SubscribeResponse) bool {
		a := m.GetAccountChanged().GetAccount()
		return a.GetProfile() == profile && a.GetState() == hearthv1.AccountState_ACCOUNT_STATE_SIGNED_OUT
	})
	if n := len(unary(t, c.prov.ListAccounts, &hearthv1.ListAccountsRequest{}).GetAccounts()); n != 4 {
		t.Errorf("ListAccounts after clear has %d accounts, want the four built-ins", n)
	}

	msgs := []proto.Message{set, list, changed}
	for _, ev := range append(added, cleared...) {
		msgs = append(msgs, ev)
	}
	for _, m := range msgs {
		if strings.Contains(prototext.Format(m), key) {
			t.Errorf("a response or event carries the key: %s", m)
		}
	}
}

func TestProviderProfile_Refusals(t *testing.T) {
	const key = "gw-0123456789-secret-wxyz"
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	ctx := t.Context()
	compat := hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE
	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"no address", func() error {
			_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{Provider: compat, Key: key, DisplayName: "G", DefaultModel: "m"}))
			return err
		}, connect.CodeInvalidArgument},
		{"plain http to another machine", func() error {
			_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{Provider: compat, Key: key, DisplayName: "G", BaseUrl: "http://g.example/v1", DefaultModel: "m"}))
			return err
		}, connect.CodeInvalidArgument},
		{"a profile that is not there", func() error {
			_, err := c.prov.SetAPIKey(ctx, connect.NewRequest(&hearthv1.SetAPIKeyRequest{Provider: compat, Profile: "deadbeef", Key: key, DisplayName: "G", BaseUrl: "https://g.example/v1", DefaultModel: "m"}))
			return err
		}, connect.CodeNotFound},
		{"clear with no profile", func() error {
			_, err := c.prov.ClearCredential(ctx, connect.NewRequest(&hearthv1.ClearCredentialRequest{Provider: compat}))
			return err
		}, connect.CodeInvalidArgument},
		{"clear a profile that is not there", func() error {
			_, err := c.prov.ClearCredential(ctx, connect.NewRequest(&hearthv1.ClearCredentialRequest{Provider: compat, Profile: "deadbeef"}))
			return err
		}, connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if got := code(err); got != tt.want {
				t.Errorf("code = %v (%v), want %v", got, err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), key) {
				t.Errorf("the error carries the key: %v", err)
			}
		})
	}
}

func TestListKnownServices(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	services := unary(t, c.prov.ListKnownServices, &hearthv1.ListKnownServicesRequest{}).GetServices()
	kinds := map[hearthv1.KnownServiceKind]int{}
	for _, s := range services {
		if s.GetId() == "" || s.GetName() == "" || s.GetBaseUrl() == "" || s.GetHelpUrl() == "" {
			t.Errorf("row %v is incomplete", s)
		}
		kinds[s.GetKind()]++
	}
	if kinds[hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_UNSPECIFIED] != 0 || kinds[hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_BUILTIN] != 4 ||
		kinds[hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_COMPATIBLE] == 0 || kinds[hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_LOCAL] == 0 {
		t.Errorf("kinds = %v", kinds)
	}
}
