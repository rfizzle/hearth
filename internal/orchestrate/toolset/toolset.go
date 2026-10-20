// Package toolset is the set of tools a run may be offered, assembled from
// the tool packages so a new tool adds no import to the orchestrator, and
// the door the agent loop calls them through.
//
// Responsibility: list every tool configured now, in a fixed order, and
// run the loop's calls through the dispatcher. Which tools a bot is offered,
// and whether a call runs, is the dispatcher's, from the bot's grants.
//
// It imports agent, the bus, org/memory, tools and the tool packages below
// it.
package toolset

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/ask"
	"github.com/rfizzle/hearth/internal/tools/files"
	notes "github.com/rfizzle/hearth/internal/tools/memory"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// Registry is every tool configured now: the file tools, asking the owner,
// web.fetch when there is a fetcher and web.search when there is a
// searcher, then extra.
func Registry(fetcher *web.Fetcher, searcher web.Searcher, extra ...tools.Definition) (*tools.Registry, error) {
	defs := append(files.Definitions(), ask.Definition())
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
