package rpc

import (
	"net/http"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/rpc/accounts"
	"github.com/rfizzle/hearth/internal/rpc/books"
	"github.com/rfizzle/hearth/internal/rpc/deliverables"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
	"github.com/rfizzle/hearth/internal/rpc/staff"
	"github.com/rfizzle/hearth/internal/store"
	"github.com/rfizzle/hearth/internal/workspace"
)

const (
	// defaultInterval is the pause between streamed words.
	defaultInterval = 50 * time.Millisecond
	defaultWords    = "Hello from hearthd, the Go core of hearth, streaming one word at a time to the app."
)

// What the owner asks a bot to be drafted from, and the proposal that comes
// back: staff's, named here because hearthd builds the Drafter.
type (
	DraftRequest = staff.DraftRequest
	Draft        = staff.Draft
	Drafter      = staff.Drafter

	ProjectDraft   = staff.ProjectDraft
	ProjectDrafter = staff.ProjectDrafter
)

// ConversationChanged is a conversation as it is now: its newest run, its
// title or its summary changed. The orchestrator publishes it too.
type ConversationChanged = orchestrate.ConversationChanged

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
	ReservedFolders workspace.Reserved
	// Drafter proposes a bot from the owner's words; BotService.DraftBot
	// asks it. Unset, a draft is unavailable.
	Drafter Drafter
	// ProjectDrafter proposes a project from the owner's paragraph;
	// ProjectService.DraftProject asks it. Unset, a draft is unavailable.
	ProjectDrafter ProjectDrafter
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
	// Deliverables backs MeasureService and DeliverableService: where the
	// figures are read from and where the Hearth folder is.
	Deliverables deliverables.Deps
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
// launch-token check. It serves the spike service and the services over the
// domain in Options: the org, bot, conversation, inbox and event ones
// through staff, the provider and integration ones through accounts, the
// handbook and usage ones through books, the measure and template ones
// through deliverables. It points the accounts' and the
// integrations' change hooks at the bus, so a sign-in finishing or a key
// pasted reaches the app whoever caused it; call it before either is used.
// See docs/architecture.md#1-layering
func NewHandler(o Options) http.Handler {
	o = o.withDefaults()
	mux := http.NewServeMux()
	deps := books.Deps{Store: o.Store, Accounts: o.Accounts, Pages: o.Handbook, Meter: o.Meter, Publish: o.publish}
	staff.Register(mux, staff.Deps{
		Version: o.Version, Interval: o.Interval, Words: o.Words,
		Store: o.Store, Orchestrator: o.Orchestrator, Templates: o.Templates,
		ReservedFolders: o.ReservedFolders, Drafter: o.Drafter, ProjectDrafter: o.ProjectDrafter, Bus: o.Bus,
		EnsureWorkspace: o.EnsureWorkspace, Books: deps, Events: accounts.Event, Publish: o.publish,
	})
	accounts.Register(mux, accounts.Deps{Accounts: o.Accounts, Integrations: o.Integrations, Books: deps, Publish: o.publish})
	mux.Handle(hearthv1connect.NewHandbookServiceHandler(books.Handbook{Deps: deps}))
	mux.Handle(hearthv1connect.NewUsageServiceHandler(books.Usage{Deps: deps}))
	deliverables.Register(mux, o.Deliverables)
	return requireToken(o.Token, mux)
}
