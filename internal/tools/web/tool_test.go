package web

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/tools"
)

func fetchExecute(t *testing.T, replies map[string]reply) func(string) (tools.Result, error) {
	t.Helper()
	defs := Definitions(newFetcher(resolver, &fakeTransport{replies: replies}), nil)
	return func(url string) (tools.Result, error) {
		args, _ := json.Marshal(map[string]string{"url": url})
		return defs[0].Execute(context.Background(), tools.Call{Args: args})
	}
}

func TestFetchTool(t *testing.T) {
	long := strings.Repeat("word ", fetchCap/5+100)
	replies := map[string]reply{
		"https://example.org/article": html200(`<html><head><title>Solar 2026</title><style>p{color:red}</style></head><body>
			<header>Site name</header><nav><a href="/">Home</a> <a href="/about">About</a></nav>
			<main><h1>Incentives</h1><p>The   credit is
			30%.</p><script>track()</script><p>It ends in <b>2032</b>.</p></main>
			<footer>Copyright</footer></body></html>`),
		"https://example.org/app":   html200(`<html><head><script>render()</script></head><body><script>more()</script><noscript>Enable JavaScript</noscript><svg><script>svg()</script></svg><!-- a comment --></body></html>`),
		"https://example.org/long":  {ctype: "text/plain", body: []byte(long)},
		"https://example.org/big":   {ctype: "text/plain", body: []byte(strings.Repeat("b", 2<<20))},
		"https://example.org/data":  {ctype: "application/json", body: []byte(`{"a":1}`)},
		"https://example.org/x.pdf": {ctype: "application/pdf", body: []byte("%PDF-1.7")},
		"https://example.org/bare":  {body: []byte("<!doctype html><p>sniffed</p>")},
	}
	cases := []struct {
		name   string
		url    string
		text   string // the page's text, after the URL line and before any notice
		notice string // a fragment of the notice; "" for none
		err    string
	}{
		{"HTML reduced to text", "https://example.org/article", "Solar 2026\n\nIncentives\n\nThe credit is 30%.\n\nIt ends in 2032.", "", ""},
		{"a page of script is empty text with a notice", "https://example.org/app", "", "no readable text", ""},
		{"a long page is cut at 48 KB", "https://example.org/long", long[:fetchCap], "cut at 48 KB", ""},
		{"a body cut at 1 MiB says so", "https://example.org/big", strings.Repeat("b", fetchCap), "only its first 1 MB", ""},
		{"JSON as it came", "https://example.org/data", `{"a":1}`, "", ""},
		{"HTML with no content type", "https://example.org/bare", "sniffed", "", ""},
		{"a PDF is refused", "https://example.org/x.pdf", "", "", "application/pdf, not a page of text"},
		{"a refused address", "http://10.0.0.1/", "", "", "web.fetch: 10.0.0.1 is a private address"},
	}
	execute := fetchExecute(t, replies)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := execute(tc.url)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("web.fetch: %v", err)
			}
			head := "URL: " + tc.url + "\n\n" + tc.text
			if tc.notice == "" && res.Text != head {
				t.Fatalf("text = %q, want %q", res.Text, head)
			}
			if tc.notice != "" {
				rest, ok := strings.CutPrefix(res.Text, head+"\n\n[")
				if !ok || !strings.Contains(rest, tc.notice) {
					t.Fatalf("text = %q, want the text then a notice with %q", tail(res.Text), tc.notice)
				}
			}
		})
	}
}

func tail(s string) string {
	if len(s) > 200 {
		return "…" + s[len(s)-200:]
	}
	return s
}

func TestDefinitions(t *testing.T) {
	fetcher := newFetcher(resolver, &fakeTransport{})
	none := Definitions(fetcher, nil)
	with := Definitions(fetcher, &FakeSearcher{})
	if names(none) != "web.fetch" || names(with) != "web.fetch web.search" {
		t.Fatalf("without a searcher %q, with one %q", names(none), names(with))
	}
	if !strings.Contains(none[0].Description, "You have no web search") {
		t.Fatal("without a searcher the fetch description does not say to start from known sites")
	}
	if strings.Contains(with[0].Description, "no web search") {
		t.Fatal("with a searcher the fetch description still says there is none")
	}
	if names(Definitions(nil, nil)) != "" {
		t.Fatal("a nil fetcher still defined web.fetch")
	}
	for _, d := range with {
		if !json.Valid(d.Parameters) || d.Tier != tools.TierRead {
			t.Errorf("%s: parameters valid %v, tier %d", d.Name, json.Valid(d.Parameters), d.Tier)
		}
	}
}

func names(defs []tools.Definition) string {
	var out []string
	for _, d := range defs {
		out = append(out, d.Name)
	}
	return strings.Join(out, " ")
}

// TestDispatcher_WebSearch is the search tool behind the grant check: it is
// offered only when a searcher is configured and the bot is granted it.
func TestDispatcher_WebSearch(t *testing.T) {
	fetcher := newFetcher(resolver, &fakeTransport{})
	both := []org.Grant{{Capability: org.WebFetch, Mode: org.ModeAllowed}, {Capability: org.WebSearch, Mode: org.ModeAllowed}}
	fetchOnly := []org.Grant{{Capability: org.WebFetch, Mode: org.ModeAllowed}}
	cases := []struct {
		name     string
		searcher Searcher
		grants   []org.Grant
		listed   bool
		err      string
	}{
		{"no searcher", nil, both, false, "no such tool"},
		{"a searcher and no grant", &FakeSearcher{}, fetchOnly, false, "not allowed"},
		{"a searcher and a grant", &FakeSearcher{Hits: []Hit{{Title: "T", URL: "https://example.org", Snippet: "S"}}}, both, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg, err := tools.NewRegistry(Definitions(fetcher, tc.searcher)...)
			if err != nil {
				t.Fatal(err)
			}
			d := tools.Dispatcher{Defs: reg, Grants: tc.grants}
			listed := slices.ContainsFunc(d.Tools(), func(def tools.Definition) bool { return def.Name == "web.search" })
			if listed != tc.listed {
				t.Fatalf("web.search listed %v, want %v", listed, tc.listed)
			}
			res, err := d.Execute(context.Background(), "web.search", json.RawMessage(`{"query":"solar"}`))
			if tc.err != "" {
				if !errors.Is(err, tools.ErrRefused) || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want a refusal with %q", err, tc.err)
				}
				return
			}
			if err != nil || !strings.Contains(res.Text, "https://example.org") {
				t.Fatalf("web.search = %q, %v", res.Text, err)
			}
		})
	}
}

func TestDescribeFetch(t *testing.T) {
	cases := map[string]string{
		`{"url":"https://example.org/page"}`:         "Fetch example.org/page",
		`{"url":"https://example.org/"}`:             "Fetch example.org",
		`{"url":"https://example.org/a?token=x#frag"}`: "Fetch example.org/a",
		`{"url":"not a url"}`:                        "Fetch not a url",
		`nonsense`:                                   "Fetch ",
	}
	for args, want := range cases {
		if got := describeFetch(json.RawMessage(args)); got != want {
			t.Errorf("describeFetch(%s) = %q, want %q", args, got, want)
		}
	}
}
