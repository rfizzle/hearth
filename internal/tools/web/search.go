package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

const (
	maxHits      = 10
	defaultHits  = 5
	searchCap    = 16 << 10
	searchMaxLen = 400
)

// Searcher is a web search service. Search returns at most count hits for
// query, best first.
type Searcher interface {
	Search(ctx context.Context, query string, count int) ([]Hit, error)
}

// Hit is one search result: what the service says the page is called, where
// it is, and a few words from it.
type Hit struct {
	Title   string
	URL     string
	Snippet string
}

type searchArgs struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

func searchDefinition(s Searcher) tools.Definition {
	return tools.Definition{
		Name: string(org.WebSearch),
		Description: "Search the web. Use it to find sources you do not already know. " +
			"Each hit is a title, a URL and a snippet, not the page: you must fetch a hit's URL to read the page before you rely on it. " +
			"Search with a few specific words, and search again with different words when the hits miss.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","description":"What to search for, in a few specific words."},` +
			`"count":{"type":"integer","description":"How many hits to return, 1 to 10. Leave it out for 5; ask for more only when the first hits are not enough."}` +
			`},"required":["query"]}`),
		Capability: org.WebSearch,
		Tier:       tools.TierRead,
		Execute: func(ctx context.Context, call tools.Call) (tools.Result, error) {
			var a searchArgs
			if err := json.Unmarshal(call.Args, &a); err != nil || strings.TrimSpace(a.Query) == "" {
				return tools.Result{}, errors.New("web.search: query is required")
			}
			if len(a.Query) > searchMaxLen {
				return tools.Result{}, fmt.Errorf("web.search: the query is longer than %d characters; use fewer words", searchMaxLen)
			}
			count := a.Count
			if count < 1 {
				count = defaultHits
			}
			count = min(count, maxHits)
			hits, err := s.Search(ctx, a.Query, count)
			if err != nil {
				return tools.Result{}, fmt.Errorf("web.search: %w", err)
			}
			return tools.Result{Text: searchResult(a.Query, hits[:min(len(hits), count)])}, nil
		},
		Describe: describeSearch,
	}
}

// searchResult lists hits, numbered, as whole hits within the cap.
func searchResult(query string, hits []Hit) string {
	if len(hits) == 0 {
		return fmt.Sprintf("No hits for %q. Try different words.", query)
	}
	var b strings.Builder
	for i, h := range hits {
		entry := fmt.Sprintf("%d. %s\n   %s\n   %s\n\n", i+1, oneLine(h.Title), oneLine(h.URL), oneLine(h.Snippet))
		if b.Len()+len(entry) > searchCap {
			fmt.Fprintf(&b, "[%d more hits were left out to stay under %d KB.]", len(hits)-i, searchCap>>10)
			return b.String()
		}
		b.WriteString(entry)
	}
	return strings.TrimSuffix(b.String(), "\n\n")
}

// oneLine is s with its white space folded to single spaces, so a hit's
// fields cannot break the numbered layout.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func describeSearch(args json.RawMessage) string {
	var a searchArgs
	_ = json.Unmarshal(args, &a) // a describer never fails; an unreadable call shows a bare verb
	return fmt.Sprintf("Search the web for '%s'", oneLine(a.Query))
}
