package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

// fetchCap is the most text one web.fetch call returns.
const fetchCap = 48 << 10

const fetchDescription = "Fetch a public web page and read it as text. " +
	"Use it to read a page whose URL you have, from a search hit, the owner or another page's link. " +
	"It reads http and https pages on the public internet only; addresses on this Mac or the local network are refused. " +
	"Scripts, menus, headers and footers are left out, and the text is cut at 48 KB."

const noSearchDescription = " You have no web search, so start from sites you know, such as an official site, " +
	"a well-known reference or a news outlet, and follow their links."

// Definitions is web.fetch over fetcher and, only when searcher is not nil,
// web.search over searcher. A nil fetcher leaves web.fetch out.
func Definitions(fetcher *Fetcher, searcher Searcher) []tools.Definition {
	var defs []tools.Definition
	if fetcher != nil {
		description := fetchDescription
		if searcher == nil {
			description += noSearchDescription
		}
		defs = append(defs, tools.Definition{
			Name:        string(org.WebFetch),
			Description: description,
			Parameters: json.RawMessage(`{"type":"object","properties":{` +
				`"url":{"type":"string","description":"The page's full URL, starting with https:// or http://."}` +
				`},"required":["url"]}`),
			Capability: org.WebFetch,
			Tier:       tools.TierRead,
			Execute:    fetchTool(fetcher),
			Describe:   describeFetch,
		})
	}
	if searcher != nil {
		defs = append(defs, searchDefinition(searcher))
	}
	return defs
}

type fetchArgs struct {
	URL string `json:"url"`
}

func fetchTool(f *Fetcher) func(context.Context, tools.Call) (tools.Result, error) {
	return func(ctx context.Context, call tools.Call) (tools.Result, error) {
		var a fetchArgs
		if err := json.Unmarshal(call.Args, &a); err != nil || a.URL == "" {
			return tools.Result{}, errors.New("web.fetch: url is required")
		}
		page, err := f.Fetch(ctx, a.URL)
		if err != nil {
			return tools.Result{}, fmt.Errorf("web.fetch: %w", err)
		}
		text, err := pageText(page)
		if err != nil {
			return tools.Result{}, fmt.Errorf("web.fetch: %w", err)
		}
		return tools.Result{Text: fetchResult(page, text)}, nil
	}
}

// fetchResult is the text the model reads: where the page was read from,
// its text within the cap, and a notice for whatever was left out.
func fetchResult(page Page, text string) string {
	var notes []string
	if page.Truncated {
		notes = append(notes, "The page is longer than 1 MB; only its first 1 MB was read.")
	}
	if strings.TrimSpace(text) == "" {
		notes = append(notes, "The page has no readable text. It may build its content with scripts, which web.fetch does not run; try another source.")
	}
	full := len(text)
	if shown, cut := cut(text, fetchCap); cut {
		text = shown
		notes = append(notes, fmt.Sprintf("The text was cut at %d KB of %d KB.", fetchCap>>10, (full+1023)>>10))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "URL: %s\n\n%s", page.URL, text)
	for _, n := range notes {
		fmt.Fprintf(&b, "\n\n[%s]", n)
	}
	return b.String()
}

// describeFetch is "Fetch example.org/page": the URL without its scheme.
func describeFetch(args json.RawMessage) string {
	var a fetchArgs
	_ = json.Unmarshal(args, &a) // a describer never fails; an unreadable call shows a bare verb
	u, err := url.Parse(a.URL)
	if err != nil || u.Host == "" {
		return "Fetch " + a.URL
	}
	return "Fetch " + strings.TrimSuffix(u.Host+u.EscapedPath(), "/")
}
