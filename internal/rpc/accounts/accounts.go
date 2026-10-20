package accounts

import (
	"net/http"

	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/rpc/books"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
)

// Deps is what the services here are handed. Publish tells the bus about a
// change and is never nil.
type Deps struct {
	// Accounts backs ProviderService.
	Accounts *provider.Accounts
	// Integrations backs IntegrationService.
	Integrations *integration.Integrations
	// Books is what the tiers and the models on offer, which ProviderService
	// takes from books, are handed.
	Books   books.Deps
	Publish func(body any)
}

// The services, one per file beside this one.
type (
	providerService struct { // ProviderService: which model accounts are signed in, and their credentials
		books.Models // the tiers and the models on offer
		opts         Deps
	}
	integrationService struct{ opts Deps } // IntegrationService: the optional services that are not model providers
)

// Register points the accounts' and the integrations' change hooks at the
// bus, so a sign-in finishing or a key pasted reaches the app whoever caused
// it, and serves ProviderService and IntegrationService on mux. Call it
// before either is used.
func Register(mux *http.ServeMux, d Deps) {
	if d.Accounts != nil {
		d.Accounts.OnChange = func(a provider.Account) { d.Publish(AccountChanged{Account: a}) }
		d.Accounts.OnCredentialChange = func(name string, deleted bool) {
			d.Publish(CredentialChanged{Name: name, Deleted: deleted})
		}
	}
	if d.Integrations != nil {
		d.Integrations.OnChange = func(a integration.Account) { d.Publish(IntegrationChanged{Integration: a}) }
	}
	mux.Handle(hearthv1connect.NewProviderServiceHandler(&providerService{Models: books.Models{Deps: d.Books}, opts: d}))
	mux.Handle(hearthv1connect.NewIntegrationServiceHandler(&integrationService{opts: d}))
}
