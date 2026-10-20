package staff

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/rpc/books"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1/hearthv1connect"
	"github.com/rfizzle/hearth/internal/store"
)

const (
	// defaultCount is how many words StreamHello sends when asked for none.
	defaultCount = 10
	// maxCount caps how many words one StreamHello may ask for. The count is
	// request input and every word holds the stream open for an interval, so
	// without a cap one caller could pin a handler for as long as it liked.
	// 1,000 words is far more than the demo stream needs and ends within a
	// minute at the default pace.
	maxCount = 1000
)

// Deps is what the services here are handed. Publish tells the bus about a
// change and is never nil; Interval and Words are never empty.
type Deps struct {
	// Version is what Ping reports.
	Version string
	// Interval is the pause between streamed words.
	Interval time.Duration
	// Words is the list StreamHello cycles through.
	Words []string

	// Store backs OrgService, BotService, ConversationService,
	// ProjectService and InboxService.
	Store *store.DB
	// Orchestrator starts and stops the runs ConversationService sends.
	Orchestrator *orchestrate.Service
	// Templates are the role templates BotService lists.
	Templates []org.RoleTemplate
	// ReservedFolders are the folders hearth keeps its own files in; a grant
	// that contains or lies inside one is refused.
	ReservedFolders []string
	// Drafter proposes a bot from the owner's words; unset, a draft is
	// unavailable.
	Drafter Drafter
	// Bus is what EventService streams.
	Bus *orchestrate.Bus
	// EnsureWorkspace creates the Hearth folder at a path UpdateOrg is
	// given, or says why it cannot; UpdateOrg refuses while it is unset.
	EnsureWorkspace func(path string) error
	// Books is what BotService takes on from books: a bot's settings and its
	// notes.
	Books books.Deps
	// Events converts a bus body the services here do not know, and says
	// whether it did; the accounts' bodies come this way. It may be nil.
	Events  func(body any) (*hearthv1.SubscribeResponse, bool)
	Publish func(body any)
}

// The services, one per file beside this one.
type (
	spike      struct{ opts Deps } // the spike: Ping and StreamHello
	orgService struct{ opts Deps } // OrgService: the org's name and the Hearth folder
	botService struct {            // BotService: the templates, hiring, listing bots and their grants
		books.Bots // the bot's settings and its notes
		opts       Deps
	}
	conversationService struct{ opts Deps } // ConversationService: the threads and the owner's side of a run
	projectService      struct{ opts Deps } // ProjectService: outcomes that take more than one task
	inboxService        struct{ opts Deps } // InboxService: what waits on the owner
	eventService        struct{ opts Deps } // EventService: the one stream every change travels on
)

// Register serves the spike, OrgService, BotService, ConversationService,
// ProjectService, InboxService and EventService on mux.
func Register(mux *http.ServeMux, d Deps) {
	mux.Handle(hearthv1connect.NewHearthServiceHandler(&spike{opts: d}))
	mux.Handle(hearthv1connect.NewOrgServiceHandler(&orgService{opts: d}))
	mux.Handle(hearthv1connect.NewBotServiceHandler(&botService{Bots: books.Bots{Deps: d.Books}, opts: d}))
	mux.Handle(hearthv1connect.NewConversationServiceHandler(&conversationService{opts: d}))
	mux.Handle(hearthv1connect.NewProjectServiceHandler(&projectService{opts: d}))
	mux.Handle(hearthv1connect.NewEventServiceHandler(&eventService{opts: d}))
	mux.Handle(hearthv1connect.NewInboxServiceHandler(&inboxService{opts: d}))
}

func (s *spike) Ping(context.Context, *connect.Request[hearthv1.PingRequest]) (*connect.Response[hearthv1.PingResponse], error) {
	return connect.NewResponse(&hearthv1.PingResponse{Version: s.opts.Version}), nil
}

func (s *spike) StreamHello(ctx context.Context, req *connect.Request[hearthv1.HelloRequest], stream *connect.ServerStream[hearthv1.HelloChunk]) error {
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
