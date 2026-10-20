package web

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// clock is a fake clock: sleeping moves it on and takes no time.
type clock struct {
	t      time.Time
	sleeps []time.Duration
}

func newClock() *clock { return &clock{t: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)} }

func (c *clock) budget() *Budget {
	b := NewBudget()
	b.now = func() time.Time { return c.t }
	b.sleep = func(_ context.Context, d time.Duration) error {
		c.sleeps = append(c.sleeps, d)
		c.t = c.t.Add(d)
		return nil
	}
	return b
}

func TestFetch_PacesOneHostToOneRequestASecond(t *testing.T) {
	c := newClock()
	tr := &fakeTransport{replies: map[string]reply{
		"https://example.org/a":   html200("a"),
		"https://example.org/b":   html200("b"),
		"https://other.example/c": html200("c"),
	}}
	f := testFetcher(tr)
	f.budget = c.budget()
	for _, u := range []string{"https://example.org/a", "https://example.org/b", "https://other.example/c"} {
		if _, err := f.Fetch(context.Background(), u); err != nil {
			t.Fatalf("Fetch(%s): %v", u, err)
		}
	}
	if want := []time.Duration{time.Second}; !slices.Equal(c.sleeps, want) {
		t.Errorf("waited %v, want %v: one wait for the second request to example.org and none for other.example", c.sleeps, want)
	}
	if len(tr.sent) != 3 {
		t.Errorf("%d requests, want 3", len(tr.sent))
	}
}

func TestFetch_RefusesThe41stFetchOfARun(t *testing.T) {
	tr := &fakeTransport{replies: map[string]reply{"https://example.org/a": html200("a")}}
	f := testFetcher(tr)
	for i := range maxFetches {
		if _, err := f.Fetch(context.Background(), "https://example.org/a"); err != nil {
			t.Fatalf("fetch %d: %v", i+1, err)
		}
	}
	_, err := f.Fetch(context.Background(), "https://example.org/a")
	if err == nil || !strings.Contains(err.Error(), "40 fetches") || strings.Contains(err.Error(), "\n") {
		t.Fatalf("the 41st fetch: %v, want a one-line refusal naming the cap", err)
	}
	if len(tr.sent) != maxFetches {
		t.Errorf("%d requests, want %d: the refused fetch must not be sent", len(tr.sent), maxFetches)
	}

	run := f.ForRun()
	if _, err := run.Fetch(context.Background(), "https://example.org/a"); err != nil {
		t.Errorf("a fresh run's first fetch: %v", err)
	}
	if _, err := f.Fetch(context.Background(), "https://example.org/a"); err == nil {
		t.Error("a new run's budget must not reset the old run's")
	}
	if (*Fetcher)(nil).ForRun() != nil {
		t.Error("a nil fetcher must stay nil")
	}
}

func TestFetch_RefusesADowngradeRedirect(t *testing.T) {
	tr := &fakeTransport{replies: map[string]reply{
		"https://example.org/a": {status: http.StatusFound, location: "http://example.org/b"},
		"http://example.org/b":  html200("plain"),
		"http://example.org/c":  {status: http.StatusFound, location: "https://example.org/d"},
		"https://example.org/d": html200("upgraded"),
	}}
	f := testFetcher(tr)
	_, err := f.Fetch(context.Background(), "https://example.org/a")
	if err == nil || !strings.Contains(err.Error(), "not secure") {
		t.Fatalf("an https to http redirect: %v, want a refusal", err)
	}
	if len(tr.sent) != 1 {
		t.Errorf("%d requests, want 1: the http hop must not be requested", len(tr.sent))
	}
	// A plain http start, and an upgrade to https, are not downgrades.
	if p, err := f.Fetch(context.Background(), "http://example.org/c"); err != nil || string(p.Body) != "upgraded" {
		t.Errorf("http to https: %q, %v", p.Body, err)
	}
}

func TestFetchTool_DecodesTheResponsesCharset(t *testing.T) {
	latin1 := []byte("<html><body><p>caf\xe9 cr\xe8me</p></body></html>")
	run := fetchExecute(t, map[string]reply{
		"https://example.org/header": {ctype: "text/html; charset=iso-8859-1", body: latin1},
		"https://example.org/meta": {ctype: "text/html", body: []byte(
			"<html><head><meta charset=\"windows-1252\"></head><body><p>caf\xe9</p></body></html>")},
		"https://example.org/text": {ctype: "text/plain; charset=ISO-8859-1", body: []byte("na\xefve")},
	})
	for url, want := range map[string]string{
		"https://example.org/header": "café crème",
		"https://example.org/meta":   "café",
		"https://example.org/text":   "naïve",
	} {
		res, err := run(url)
		if err != nil || !strings.Contains(res.Text, want) {
			t.Errorf("%s = %q, %v; want it to read %q", url, res.Text, err, want)
		}
	}
}
