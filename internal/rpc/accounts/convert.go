package accounts

import (
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// The conversions for the accounts. A provider's name and an integration's
// name differ in kind from the contract's enum, so each goes through a table.

// integrationNames is the contract's integration enum against integration's
// names.
var integrationNames = map[hearthv1.Integration]integration.Name{
	hearthv1.Integration_INTEGRATION_BRAVE_SEARCH: integration.BraveSearch,
}

var accountStates = map[provider.State]hearthv1.AccountState{
	provider.SignedOut: hearthv1.AccountState_ACCOUNT_STATE_SIGNED_OUT,
	provider.SigningIn: hearthv1.AccountState_ACCOUNT_STATE_SIGNING_IN,
	provider.SignedIn:  hearthv1.AccountState_ACCOUNT_STATE_SIGNED_IN,
}

// accountToProto carries a label and a failure, never a credential: the
// provider's Account holds none.
func accountToProto(a provider.Account) *hearthv1.ProviderAccount {
	return &hearthv1.ProviderAccount{
		Provider:     wire.ToProto(wire.ProviderNames, a.Provider),
		State:        accountStates[a.State],
		Label:        a.Label,
		Failure:      a.Failure,
		Profile:      a.Profile,
		DisplayName:  a.DisplayName,
		BaseUrl:      a.BaseURL,
		DefaultModel: a.DefaultModel,
	}
}

// integrationAccountToProto carries the label (the last four characters)
// and never the key, like the provider's.
func integrationAccountToProto(a integration.Account) *hearthv1.IntegrationAccount {
	kind := hearthv1.IntegrationKind_INTEGRATION_KIND_UNSPECIFIED
	if a.Kind == integration.WebSearch {
		kind = hearthv1.IntegrationKind_INTEGRATION_KIND_WEB_SEARCH
	}
	return &hearthv1.IntegrationAccount{
		Integration: wire.ToProto(integrationNames, a.Name),
		Kind:        kind,
		Name:        a.Title,
		Configured:  a.Configured,
		Label:       a.Label,
		HelpUrl:     a.HelpURL,
	}
}
