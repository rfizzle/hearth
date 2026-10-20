// Package toolset is the set of tools a run may be offered, assembled from
// the tool packages, so a new tool adds no import to the orchestrator.
//
// Responsibility: list every tool configured now, in a fixed order. Which
// of them a bot is offered is the dispatcher's, from the bot's grants.
//
// It imports tools and the tool packages below it, and nothing that runs.
package toolset

import (
	"github.com/rfizzle/hearth/internal/tools"
	"github.com/rfizzle/hearth/internal/tools/ask"
	"github.com/rfizzle/hearth/internal/tools/files"
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
