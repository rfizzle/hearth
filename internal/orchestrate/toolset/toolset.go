// Package toolset is the set of tools a run may be offered, assembled from
// the tool packages so a new tool adds no import to the orchestrator, and
// the door the agent loop calls them through.
//
// Responsibility: list every tool configured now, in a fixed order, and
// run the loop's calls through the dispatcher. Which tools a bot is offered,
// and whether a call runs, is the dispatcher's, from the bot's grants.
//
// It imports agent, the bus, org, org/memory, project, tools and the tool
// packages below it.
package toolset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/ask"
	"github.com/rfizzle/hearth/internal/tools/deliverables"
	"github.com/rfizzle/hearth/internal/tools/files"
	notes "github.com/rfizzle/hearth/internal/tools/memory"
	"github.com/rfizzle/hearth/internal/tools/propose"
	"github.com/rfizzle/hearth/internal/tools/web"
	"github.com/rfizzle/hearth/internal/workspace"
)

// Registry is every tool configured now: the file tools, asking the owner,
// web.fetch when there is a fetcher and web.search when there is a
// searcher, then extra. docs says how documents are read.
func Registry(docs files.Options, fetcher *web.Fetcher, searcher web.Searcher, extra ...tools.Definition) (*tools.Registry, error) {
	defs := append(files.Definitions(docs), ask.Definition())
	defs = append(defs, web.Definitions(fetcher, searcher)...)
	return tools.NewRegistry(append(defs, extra...)...)
}

// Notes is where remember writes a bot's notes; the store's *DB is one.
type Notes interface {
	AddNote(ctx context.Context, n memory.Note) (memory.Note, error)
	Notes(ctx context.Context, botID string) ([]memory.Note, error)
}

// Remember is the remember tool for the bot botID in the conversation
// convID, linking each note to the message msgID, writing it to db and
// publishing it on b.
// See docs/model.md#7-memory-writes.
func Remember(db Notes, b *bus.Bus, botID, convID, msgID string) tools.Definition {
	return notes.Definition(notes.Store{
		BotID: botID, ConversationID: convID, MessageID: msgID,
		Add: func(ctx context.Context, n memory.Note) error {
			stored, err := db.AddNote(ctx, n)
			if err == nil {
				b.Publish(bus.NoteChanged{BotID: botID, Note: stored})
			}
			return err
		},
		Count: func(ctx context.Context, botID string) (int, error) {
			all, err := db.Notes(ctx, botID)
			return len(all), err
		},
	})
}

// Templates loads the deliverable templates a run is offered, and Measures
// reads the office's figures a deliverable binds.
type (
	Templates = deliverables.Loader
	Measures  = deliverables.Measures
)

// Deliverables is the deliverables.write tool for the bot botID in a
// conversation of the project projectID, empty outside a project: the
// aliases me and this mean them. templates is called once, now; no output
// lands in a reserved folder.
// See docs/decisions.md for D44.
func Deliverables(templates Templates, measures Measures, botID, projectID string, reserved workspace.Reserved) tools.Definition {
	return deliverables.Definition(deliverables.Options{
		Templates: templates, Measures: measures, BotID: botID, ProjectID: projectID, Reserved: reserved,
	})
}

// ErrAlreadyProposed refuses a proposal whose name is already waiting for the
// owner in the same conversation.
var ErrAlreadyProposed = errors.New("already proposed: a project with this name is waiting for the owner's decision in this conversation; do not propose it again")

// Projects is where a proposal is stored and found; the store's *DB is one.
type Projects interface {
	CreateProject(ctx context.Context, p project.Project, conv org.Conversation) (project.Project, error)
	Projects(ctx context.Context, includeArchived bool) ([]project.Project, error)
}

// Propose is the propose_project tool for the bot botID in the conversation
// convID. Each valid proposal is checked by project.Propose, refused with
// ErrAlreadyProposed while a proposal of the same name, trimmed, is open in
// convID, stored as a proposed project carrying botID and convID, and
// published on b with the status that status gives it. The name and brief
// are stored as the model wrote them.
// See docs/model.md#5-the-chief-of-staffs-routing-policy.
func Propose(db Projects, b *bus.Bus, status func(ctx context.Context, id string) (project.Status, error), botID, convID string) tools.Definition {
	return propose.Definition(func(ctx context.Context, name string, brief project.Brief) (project.Project, error) {
		p, err := project.Propose(name, brief, botID, convID)
		if err != nil {
			return project.Project{}, err
		}
		all, err := db.Projects(ctx, false)
		if err != nil {
			return project.Project{}, err
		}
		for _, o := range all {
			if o.State == project.Proposed && o.ProposedInConversationID == convID && strings.TrimSpace(o.Name) == p.Name {
				return project.Project{}, ErrAlreadyProposed
			}
		}
		stored, err := db.CreateProject(ctx, p, org.Conversation{})
		if err != nil {
			return project.Project{}, err
		}
		st, err := status(ctx, stored.ID)
		if err != nil {
			return project.Project{}, err
		}
		b.Publish(bus.ProjectChanged{Project: stored, Status: st})
		return stored, nil
	})
}

// Execute runs a call through d, the one place its grants are checked, and
// hands the loop the paths of the files it wrote. A panic in the call is
// handed to crashed, which stops the run, and the loop reads that the tool
// stopped.
func Execute(d tools.Dispatcher, crashed func(any)) agent.Execute {
	return func(ctx context.Context, name string, args json.RawMessage) (res agent.Result, err error) {
		defer func() {
			if p := recover(); p != nil {
				crashed(p)
				err = fmt.Errorf("%s stopped unexpectedly", name)
			}
		}()
		r, err := d.Execute(ctx, name, args)
		paths := make([]string, len(r.Files))
		for i, f := range r.Files {
			paths[i] = f.Path
		}
		return agent.Result{Text: r.Text, Files: paths}, err
	}
}
