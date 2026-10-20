package gateway

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/llm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// answering is a client whose every request gets status and body, and
// remembers the path and authorization it was asked with.
func answering(status int, body string, seen *[]string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = append(*seen, r.URL.Path+" "+r.Header.Get("Authorization"))
		}
		rec := httptest.NewRecorder()
		rec.WriteHeader(status)
		_, _ = rec.WriteString(body)
		return rec.Result(), nil
	})}
}

func TestListModelsReadsTheIds(t *testing.T) {
	var seen []string
	c := answering(200, `{"data":[{"id":"b"},{"id":""},{"id":"a"}]}`, &seen)
	ids, err := ListModels(t.Context(), c, "https://gw.example/v1", "k-1")
	if err != nil || !slices.Equal(ids, []string{"b", "a"}) {
		t.Errorf("ListModels = %q, %v", ids, err)
	}
	if want := []string{"/v1/models Bearer k-1"}; !slices.Equal(seen, want) {
		t.Errorf("the service saw %q, want %q", seen, want)
	}
}

func TestListModelsRefusalsWrapErrList(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"key refused", 401, `{"error":"bad key k-1"}`, "The key was refused."},
		{"not json", 200, `<html>`, "many gateways need `/v1`"},
		{"no data", 200, `{"x":1}`, "not as an OpenAI-compatible service"},
		{"server error", 500, `oops`, "not as an OpenAI-compatible service"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ListModels(t.Context(), answering(tt.status, tt.body, nil), "https://gw.example/v1", "k-1")
			if !errors.Is(err, ErrList) || err == nil || !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "k-1") {
				t.Errorf("ListModels error = %v, want an ErrList containing %q and no key", err, tt.want)
			}
		})
	}
	down := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, http.ErrHandlerTimeout })}
	if _, err := ListModels(t.Context(), down, "https://gw.example/v1", ""); !errors.Is(err, ErrList) {
		t.Errorf("an unreachable service = %v, want ErrList", err)
	}
}

func TestCheckBaseURL(t *testing.T) {
	for _, tt := range []struct {
		raw      string
		base     string
		local    bool
		refused  bool
		fragment string
	}{
		{raw: " https://gw.example/v1/ ", base: "https://gw.example/v1"},
		{raw: "http://localhost:11434/v1", base: "http://localhost:11434/v1", local: true},
		{raw: "http://127.0.0.1:1234/v1", base: "http://127.0.0.1:1234/v1", local: true},
		{raw: "", refused: true, fragment: "field was empty"},
		{raw: "file:///etc/passwd", refused: true, fragment: "should look like"},
		{raw: "http://gw.example/v1", refused: true, fragment: "use an https address"},
		{raw: "https://user:pw@gw.example/v1", refused: true, fragment: "not in the address"},
		{raw: "https://gw.example/v1?key=abc", refused: true, fragment: "not in the address"},
	} {
		base, local, err := CheckBaseURL(tt.raw)
		if tt.refused {
			if !errors.Is(err, ErrAddress) || !strings.Contains(err.Error(), tt.fragment) {
				t.Errorf("CheckBaseURL(%q) = %v, want an ErrAddress containing %q", tt.raw, err, tt.fragment)
			}
			continue
		}
		if err != nil || base != tt.base || local != tt.local {
			t.Errorf("CheckBaseURL(%q) = %q, %v, %v; want %q, %v", tt.raw, base, local, err, tt.base, tt.local)
		}
	}
}

func TestHostMoved(t *testing.T) {
	if HostMoved("https://a.example/v1", "https://a.example/v2") {
		t.Error("a new path on the same host is not a move")
	}
	if !HostMoved("https://a.example/v1", "https://b.example/v1") {
		t.Error("a new host is a move")
	}
}

// sends is a provider that makes one request through c and ends with
// nothing said, as a service that answers 200 to every path does.
type sends struct {
	llm.Provider
	c *http.Client
}

func (s sends) StreamCompletion(ctx context.Context, _ []llm.Message, _ llm.CompletionOpts) (<-chan llm.StreamEvent, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://gw.example/v1/chat/completions", nil)
	resp, err := s.c.Do(req)
	if err != nil {
		return nil, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	ch := make(chan llm.StreamEvent, 1)
	ch <- llm.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

func TestCheckNamesWhatAServiceSentInsteadOfAReply(t *testing.T) {
	for _, tt := range []struct {
		name, body, want string
	}{
		{"a foreign body", `{"Output":{"__type":"UnknownOperationException"}}`, "Acme answered, but not with a reply"},
		{"a completion with no words", `{"choices":[]}`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := Check(sends{c: Peeking(answering(200, tt.body, nil))}, "Acme", func(s string) string { return strings.ReplaceAll(s, "UnknownOperation", "") })
			ch, err := p.StreamCompletion(t.Context(), nil, llm.CompletionOpts{})
			if err != nil {
				t.Fatal(err)
			}
			var got error
			for ev := range ch {
				got = ev.Err
			}
			if tt.want == "" {
				if got != nil {
					t.Errorf("a body with choices ended with %v", got)
				}
				return
			}
			if got == nil || !strings.Contains(got.Error(), tt.want) || strings.Contains(got.Error(), "UnknownOperation") {
				t.Errorf("the stream ended with %v, want %q with the scrubbed body", got, tt.want)
			}
		})
	}
}
