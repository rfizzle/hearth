package accounts

import (
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The bodies published on the bus for the accounts: a change to an account
// or an integration is not a run, so it is named here.

// AccountChanged is a model provider's account in its new state.
type AccountChanged struct{ Account provider.Account }

// IntegrationChanged is an integration's account in its new state.
type IntegrationChanged struct{ Integration integration.Account }

// CredentialChanged names a credential that was stored or removed. It has no
// value field on purpose: the app fetches the record with ExportCredential.
type CredentialChanged struct {
	Name    string
	Deleted bool
}

// Event is the SubscribeResponse for a body published here, and false for
// any other body. The caller sets the sequence.
func Event(body any) (*hearthv1.SubscribeResponse, bool) {
	out := &hearthv1.SubscribeResponse{}
	switch b := body.(type) {
	case AccountChanged:
		out.Event = &hearthv1.SubscribeResponse_AccountChanged{AccountChanged: &hearthv1.AccountChanged{Account: accountToProto(b.Account)}}
	case IntegrationChanged:
		changed := &hearthv1.IntegrationChanged{Integration: integrationAccountToProto(b.Integration)}
		out.Event = &hearthv1.SubscribeResponse_IntegrationChanged{IntegrationChanged: changed}
	case CredentialChanged:
		out.Event = &hearthv1.SubscribeResponse_CredentialChanged{CredentialChanged: &hearthv1.CredentialChanged{Name: b.Name, Deleted: b.Deleted}}
	default:
		return nil, false
	}
	return out, true
}
