package rpc

import (
	"slices"
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

func TestProviderProfile_AnAddressThatIsNotAServiceIsRefused(t *testing.T) {
	const key = "gw-0123456789-secret-wxyz"
	r := newRig(t)
	r.gw.set(200, `{"Output":{"__type":"com.amazon.coral.service#UnknownOperationException"}}`)
	c := newClients(serve(t, r.opts), testToken)
	compat := hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE
	_, err := c.prov.SetAPIKey(t.Context(), connect.NewRequest(&hearthv1.SetAPIKeyRequest{
		Provider: compat, Key: key, DisplayName: "G", BaseUrl: "https://gw.example/claude", DefaultModel: "m",
	}))
	if code(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "many gateways need `/v1`") || strings.Contains(err.Error(), key) {
		t.Errorf("error = %v, want invalid_argument with the address words and no key", err)
	}
	if n := len(unary(t, c.prov.ListAccounts, &hearthv1.ListAccountsRequest{}).GetAccounts()); n != 4 {
		t.Errorf("a refused save left %d accounts, want the four built-ins", n)
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

func TestListServiceModels_BothFormsAndTheRefusals(t *testing.T) {
	const key = "gw-0123456789-secret-wxyz"
	r := newRig(t)
	r.gw.set(200, `{"data":[{"id":"claude-sonnet-5-5"},{"id":"gpt-x"}]}`)
	c := newClients(serve(t, r.opts), testToken)
	saved := unary(t, c.prov.SetAPIKey, &hearthv1.SetAPIKeyRequest{
		Provider: hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE, Key: key,
		DisplayName: "Acme gateway", BaseUrl: "https://llm.acme.example/v1", DefaultModel: "claude-sonnet-5-5",
	}).GetAccount().GetProfile()

	// Before saving: the address and the key in the request, used once.
	before := len(r.gw.asked())
	got := unary(t, c.prov.ListServiceModels, &hearthv1.ListServiceModelsRequest{BaseUrl: "https://other.example/v1/", Key: " k-new "}).GetModels()
	if len(got) != 2 || got[0] != "claude-sonnet-5-5" {
		t.Errorf("models = %v", got)
	}
	// A saved service: its stored address and key, nothing typed; the
	// other fields are ignored.
	got = unary(t, c.prov.ListServiceModels, &hearthv1.ListServiceModelsRequest{Profile: saved, BaseUrl: "https://ignored.example/v1", Key: "ignored"}).GetModels()
	if len(got) != 2 {
		t.Errorf("models of a saved service = %v", got)
	}
	want := []string{"https://other.example/v1/models Bearer k-new", "https://llm.acme.example/v1/models Bearer " + key}
	if asked := r.gw.asked()[before:]; !slices.Equal(asked, want) {
		t.Errorf("the service was asked %q, want %q", asked, want)
	}

	tests := []struct {
		name string
		req  *hearthv1.ListServiceModelsRequest
		want connect.Code
	}{
		{"plain http to another machine", &hearthv1.ListServiceModelsRequest{BaseUrl: "http://g.example/v1", Key: key}, connect.CodeInvalidArgument},
		{"no address", &hearthv1.ListServiceModelsRequest{}, connect.CodeInvalidArgument},
		{"a profile that is not there", &hearthv1.ListServiceModelsRequest{Profile: "deadbeef"}, connect.CodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := len(r.gw.asked())
			err := errOf(t, c.prov.ListServiceModels, tt.req)
			if code(err) != tt.want || (err != nil && strings.Contains(err.Error(), key)) {
				t.Errorf("code = %v (%v), want %v and no key", code(err), err, tt.want)
			}
			if n := len(r.gw.asked()); n != before {
				t.Errorf("a refused request still reached the network (%d calls)", n-before)
			}
		})
	}

	// A service that answers, but not with a model list, is unavailable
	// and says what to check.
	r.gw.set(200, `<html>sign in</html>`)
	err := errOf(t, c.prov.ListServiceModels, &hearthv1.ListServiceModelsRequest{Profile: saved})
	if code(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "many gateways need `/v1`") {
		t.Errorf("an unreadable list = %v, want unavailable with the address words", err)
	}
	r.gw.set(401, `{"error":"no"}`)
	err = errOf(t, c.prov.ListServiceModels, &hearthv1.ListServiceModelsRequest{BaseUrl: "https://llm.acme.example/v1", Key: key})
	if code(err) != connect.CodeUnavailable || !strings.Contains(err.Error(), "The key was refused.") || strings.Contains(err.Error(), key) {
		t.Errorf("a refused key = %v, want unavailable, in words, without the key", err)
	}
}
