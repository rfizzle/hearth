package rpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/rpc/books"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
	"github.com/rfizzle/hearth/internal/store"
)

const (
	// defaultInterval is the pause between streamed words.
	defaultInterval = 50 * time.Millisecond
	// defaultCount is how many words StreamHello sends when asked for none.
	defaultCount = 10
	// maxCount caps how many words one StreamHello may ask for. The count is
	// request input and every word holds the stream open for an interval, so
	// without a cap one caller could pin a handler for as long as it liked.
	// 1,000 words is far more than the demo stream needs and ends within a
	// minute at the default pace.
	maxCount     = 1000
	defaultWords = "Hello from hearthd, the Go core of hearth, streaming one word at a time to the app."
)

// Options configures the handler NewHandler returns.
type Options struct {
	// Version is what Ping reports.
	Version string
	// Token is the launch token every request must carry as a bearer token.
	Token string
	// Interval is the pause between streamed words; zero means 50 ms.
	Interval time.Duration
	// Words is the list StreamHello cycles through; empty means a fixed sentence.
	Words []string

	// The domain the services translate to. Every service is registered, so
	// a handler called without its part set fails; hearthd sets all of them,
	// and a test of the spike service sets none.

	// Store backs OrgService, BotService and ConversationService.
	Store *store.DB
	// Orchestrator starts and stops the runs ConversationService sends.
	Orchestrator *orchestrate.Service
	// Accounts backs ProviderService. NewHandler publishes its changes on
	// Bus.
	Accounts *provider.Accounts
	// Integrations backs IntegrationService. NewHandler publishes its
	// changes on Bus.
	Integrations *integration.Integrations
	// Templates are the role templates BotService lists.
	Templates []org.RoleTemplate
	// ReservedFolders are the folders hearth keeps its own files in; a grant
	// that contains or lies inside one is refused.
	ReservedFolders []string
	// Drafter proposes a bot from the owner's words; BotService.DraftBot
	// asks it. Unset, a draft is unavailable.
	Drafter Drafter
	// Bus is what EventService streams and the handlers publish to.
	Bus *orchestrate.Bus
	// EnsureWorkspace creates the Hearth folder at a path UpdateOrg is
	// given, or says why it cannot; UpdateOrg refuses while it is unset.
	EnsureWorkspace func(path string) error
	// Handbook keeps the pages HandbookService serves; it tells the bus
	// itself, with books.PageChanged.
	Handbook books.Pages
	// Meter is the usage ledger UsageService reads and sets; its ticks
	// reach the bus through books.Ticks.
	Meter books.Ledger
}

// publish tells the bus about a change a handler made; with no bus nobody
// is following.
func (o Options) publish(body any) {
	if o.Bus != nil {
		o.Bus.Publish(body)
	}
}

func (o Options) withDefaults() Options {
	if o.Interval <= 0 {
		o.Interval = defaultInterval
	}
	if len(o.Words) == 0 {
		o.Words = strings.Fields(defaultWords)
	}
	return o
}

// NewHandler returns the HTTP handler for the hearth.v1 contract, behind the
// launch-token check. It serves the spike service and the nine services over
// the domain in Options, the handbook and usage ones through books. It points the accounts' and the integrations' change
// hooks at the bus, so a sign-in finishing or a key pasted reaches the app
// whoever caused it; call it before either is used.
// See docs/architecture.md#1-layering
func NewHandler(o Options) http.Handler {
	o = o.withDefaults()
	mux := http.NewServeMux()
	mux.Handle(hearthv1connect.NewHearthServiceHandler(&service{opts: o}))
	mux.Handle(hearthv1connect.NewOrgServiceHandler(&orgService{opts: o}))
	deps := books.Deps{Store: o.Store, Accounts: o.Accounts, Pages: o.Handbook, Meter: o.Meter, Publish: o.publish}
	mux.Handle(hearthv1connect.NewBotServiceHandler(&botService{Bots: books.Bots{Deps: deps}, opts: o}))
	if o.Accounts != nil {
		o.Accounts.OnChange = func(a provider.Account) { o.publish(AccountChanged{Account: a}) }
		o.Accounts.OnCredentialChange = func(name string, deleted bool) { o.publish(CredentialChanged{Name: name, Deleted: deleted}) }
	}
	if o.Integrations != nil {
		o.Integrations.OnChange = func(a integration.Account) { o.publish(IntegrationChanged{Integration: a}) }
	}
	mux.Handle(hearthv1connect.NewConversationServiceHandler(&conversationService{opts: o}))
	mux.Handle(hearthv1connect.NewProviderServiceHandler(&providerService{Models: books.Models{Deps: deps}, opts: o}))
	mux.Handle(hearthv1connect.NewIntegrationServiceHandler(&integrationService{opts: o}))
	mux.Handle(hearthv1connect.NewEventServiceHandler(&eventService{opts: o}))
	mux.Handle(hearthv1connect.NewInboxServiceHandler(&inboxService{opts: o}))
	mux.Handle(hearthv1connect.NewHandbookServiceHandler(books.Handbook{Deps: deps}))
	mux.Handle(hearthv1connect.NewUsageServiceHandler(books.Usage{Deps: deps}))
	return requireToken(o.Token, mux)
}

// The services, one per file beside this one.
type (
	service    struct{ opts Options } // the spike: Ping and StreamHello
	orgService struct{ opts Options } // OrgService: the org's name and the Hearth folder
	botService struct {               // BotService: the templates, hiring, listing bots and their grants
		books.Bots // the bot's settings and its notes
		opts       Options
	}
	conversationService struct{ opts Options } // ConversationService: the threads and the owner's side of a run
	providerService     struct {               // ProviderService: which model accounts are signed in, and their credentials
		books.Models // the tiers and the models on offer
		opts         Options
	}
	integrationService struct{ opts Options } // IntegrationService: the optional services that are not model providers
	eventService       struct{ opts Options } // EventService: the one stream every change travels on
)

func (s *service) Ping(context.Context, *connect.Request[hearthv1.PingRequest]) (*connect.Response[hearthv1.PingResponse], error) {
	return connect.NewResponse(&hearthv1.PingResponse{Version: s.opts.Version}), nil
}

func (s *service) StreamHello(ctx context.Context, req *connect.Request[hearthv1.HelloRequest], stream *connect.ServerStream[hearthv1.HelloChunk]) error {
	count := int(req.Msg.GetCount())
	if count <= 0 {
		count = defaultCount
	}
	count = min(count, maxCount)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for i := range count {
		if i > 0 {
			timer.Reset(s.opts.Interval)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
		chunk := &hearthv1.HelloChunk{Index: int32(i), Word: s.opts.Words[i%len(s.opts.Words)]}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
	return nil
}
