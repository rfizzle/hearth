package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/tools"
)

// FakeSearcher answers every search with Hits, or Err, and remembers what
// it was asked.
type FakeSearcher struct {
	Hits  []Hit
	Err   error
	Query string
	Count int
}

func (f *FakeSearcher) Search(_ context.Context, query string, count int) ([]Hit, error) {
	f.Query, f.Count = query, count
	return f.Hits, f.Err
}

func hits(n int, snippet string) []Hit {
	out := make([]Hit, n)
	for i := range out {
		out[i] = Hit{Title: fmt.Sprintf("Page %d", i+1), URL: fmt.Sprintf("https://example.org/%d", i+1), Snippet: snippet}
	}
	return out
}

func TestSearchTool(t *testing.T) {
	cases := []struct {
		name      string
		args      string
		searcher  *FakeSearcher
		wantCount int    // the count the searcher was asked for
		want      string // a fragment of the result
		notWant   string
		err       string
	}{
		{"one hit", `{"query":"solar incentives"}`, &FakeSearcher{Hits: hits(1, "A  snippet\nover lines")}, defaultHits,
			"1. Page 1\n   https://example.org/1\n   A snippet over lines", "", ""},
		{"a count over 10 is capped", `{"query":"q","count":50}`, &FakeSearcher{Hits: hits(12, "s")}, maxHits, "10. Page 10", "11. Page 11", ""},
		{"a count asked for", `{"query":"q","count":3}`, &FakeSearcher{Hits: hits(3, "s")}, 3, "3. Page 3", "", ""},
		{"no hits", `{"query":"q"}`, &FakeSearcher{}, defaultHits, `No hits for "q"`, "", ""},
		{"the result is capped at 16 KB", `{"query":"q","count":10}`, &FakeSearcher{Hits: hits(10, strings.Repeat("x", 3000))}, maxHits,
			"more hits were left out to stay under 16 KB", "6. Page 6", ""},
		{"no query", `{"query":"  "}`, &FakeSearcher{}, 0, "", "", "query is required"},
		{"a query too long", `{"query":"` + strings.Repeat("q", searchMaxLen+1) + `"}`, &FakeSearcher{}, 0, "", "", "longer than 400"},
		{"the service fails", `{"query":"q"}`, &FakeSearcher{Err: errors.New("the key was refused")}, defaultHits, "", "", "web.search: the key was refused"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := searchDefinition(tc.searcher)
			res, err := def.Execute(context.Background(), tools.Call{Args: json.RawMessage(tc.args)})
			if tc.searcher.Count != tc.wantCount {
				t.Fatalf("the searcher was asked for %d hits, want %d", tc.searcher.Count, tc.wantCount)
			}
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("err = %v, want %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("web.search: %v", err)
			}
			if !strings.Contains(res.Text, tc.want) || (tc.notWant != "" && strings.Contains(res.Text, tc.notWant)) {
				t.Fatalf("text = %q, want %q and not %q", tail(res.Text), tc.want, tc.notWant)
			}
			if len(res.Text) > searchCap {
				t.Fatalf("text is %d bytes, over the %d cap", len(res.Text), searchCap)
			}
		})
	}
	def := searchDefinition(&FakeSearcher{})
	if got := def.Describe(json.RawMessage(`{"query":"solar incentives 2026"}`)); got != "Search the web for 'solar incentives 2026'" {
		t.Fatalf("describe = %q", got)
	}
	for _, words := range []string{"title, a URL and a snippet", "fetch a hit's URL to read"} {
		if !strings.Contains(def.Description, words) {
			t.Errorf("the description does not say %q", words)
		}
	}
}

// braveTransport answers as Brave's endpoint would and records the request.
type braveTransport struct {
	status   int
	location string
	body     []byte
	req      *http.Request
	err      error
}

func (b *braveTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	b.req = req
	if b.err != nil {
		return nil, b.err
	}
	return &http.Response{
		StatusCode: b.status, Header: http.Header{"Content-Type": {"application/json"}, "Location": {b.location}},
		Body: io.NopCloser(bytes.NewReader(b.body)), Request: req,
	}, nil
}

func TestBrave(t *testing.T) {
	fixture, err := os.ReadFile("testdata/brave_web_search.json")
	if err != nil {
		t.Fatal(err)
	}
	tr := &braveTransport{status: http.StatusOK, body: fixture}
	got, err := NewBrave("BSA-test-key", &http.Client{Transport: tr}).Search(context.Background(), "solar incentives 2026", 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	req := tr.req
	if req.Method != http.MethodGet || req.URL.Scheme+"://"+req.URL.Host+req.URL.Path != braveEndpoint {
		t.Fatalf("request %s %s", req.Method, req.URL)
	}
	if q := req.URL.Query(); q.Get("q") != "solar incentives 2026" || q.Get("count") != "3" {
		t.Fatalf("query %v", q)
	}
	if req.Header.Get("X-Subscription-Token") != "BSA-test-key" || strings.Contains(req.URL.String(), "BSA-test-key") {
		t.Fatal("the key is not in its header alone")
	}
	if deadline, ok := req.Context().Deadline(); !ok || time.Until(deadline) > braveTimeout {
		t.Fatalf("no deadline within %s", braveTimeout)
	}
	want := []Hit{
		{"Residential Clean Energy Credit | Internal Revenue Service", "https://www.irs.gov/credits-deductions/residential-clean-energy-credit",
			"The residential clean energy credit equals 30% of the costs of new, qualified clean energy property for your home installed anytime from 2022 through 2032."},
		{"Homeowner's Guide to the Federal Tax Credit for Solar Photovoltaics", "https://www.energy.gov/eere/solar/homeowners-guide-federal-tax-credit-solar-photovoltaics",
			"This guide provides an overview of the federal investment tax credit for residential solar photovoltaics (PV)."},
		{"Database of State Incentives for Renewables & Efficiency", "https://www.dsireusa.org/",
			"DSIRE is the most comprehensive source of information on incentives and policies that support renewables and energy efficiency in the United States."},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d hits, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hit %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBrave_Errors(t *testing.T) {
	tokenInvalid := `{"type":"ErrorResponse","error":{"id":"x","status":422,"code":"SUBSCRIPTION_TOKEN_INVALID","detail":"The provided subscription token is invalid."},"time":1}`
	badParam := `{"type":"ErrorResponse","error":{"id":"x","status":422,"code":"VALIDATION","detail":"Unable to validate request parameter(s)"},"time":1}`
	cases := []struct {
		name string
		tr   *braveTransport
		want string
	}{
		{"a refused key", &braveTransport{status: http.StatusUnauthorized}, "key was refused"},
		{"an invalid token", &braveTransport{status: http.StatusUnprocessableEntity, body: []byte(tokenInvalid)}, "key was refused"},
		{"a rate limit", &braveTransport{status: http.StatusTooManyRequests}, "limit for this key was reached"},
		{"a server error", &braveTransport{status: http.StatusBadGateway}, "trouble at Brave Search (status 502)"},
		{"a bad parameter", &braveTransport{status: http.StatusUnprocessableEntity, body: []byte(badParam)}, "refused by Brave Search: Unable to validate"},
		{"an unknown refusal", &braveTransport{status: http.StatusTeapot}, "refused by Brave Search (status 418)"},
		{"a redirect is not followed with the key", &braveTransport{status: http.StatusFound, location: "https://elsewhere.example/"}, "refused by Brave Search (status 302)"},
		{"not JSON", &braveTransport{status: http.StatusOK, body: []byte("<html>")}, "not a search result"},
		{"no connection", &braveTransport{err: errors.New("dial tcp: no route")}, "could not reach Brave Search"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewBrave("k", &http.Client{Transport: tc.tr}).Search(context.Background(), "q", 5)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	if NewBrave("k", nil).client == nil {
		t.Fatal("a nil client was kept")
	}
}
