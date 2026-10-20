package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/rfizzle/llm"
)

const (
	// peekBytes is how much of a response is kept to say what a service
	// sent instead of a reply.
	peekBytes = 2048
	// quoteChars is how much of it is quoted to the person.
	quoteChars = 80
)

// peekKey carries the keeper of one request's first bytes through the
// request's context to the transport.
type peekKey struct{}

// peek keeps the first bytes of a successful response body.
type peek struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (p *peek) add(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if room := peekBytes - p.buf.Len(); room > 0 {
		p.buf.Write(b[:min(room, len(b))])
	}
}

func (p *peek) text() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

// peekingTransport records the start of each 200 body for the request
// that carries a peek in its context.
type peekingTransport struct{ next http.RoundTripper }

func (t peekingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	resp, err := next.RoundTrip(r)
	if err != nil || resp.StatusCode != http.StatusOK {
		return resp, err
	}
	if p, ok := r.Context().Value(peekKey{}).(*peek); ok {
		resp.Body = &peekBody{ReadCloser: resp.Body, p: p}
	}
	return resp, nil
}

type peekBody struct {
	io.ReadCloser
	p *peek
}

func (b *peekBody) Read(buf []byte) (int, error) {
	n, err := b.ReadCloser.Read(buf)
	b.p.add(buf[:n])
	return n, err
}

// httpClient is c, or a plain client when c is nil.
func httpClient(c *http.Client) *http.Client {
	if c == nil {
		return &http.Client{}
	}
	return c
}

// peeking returns c sending through a transport that keeps response starts.
func peeking(c *http.Client) *http.Client {
	out := *httpClient(c)
	out.Transport = peekingTransport{next: out.Transport}
	return &out
}

// checked wraps an added service's provider so a completion that is not
// one is an error. A gateway can answer 200 to every path with a body of
// its own (an unknown-operation notice, say); the stream reader sees no
// choices, takes it for a ping and ends without a word, and the person is
// left with an empty reply and no reason. Here an end with nothing said,
// from a body that had no choices in it, reports the service and the first
// of what it sent, with any secret removed.
// See docs/model.md#providers-and-models
type checked struct {
	llm.Provider
	name  string
	scrub func(string) string
}

func (c checked) StreamCompletion(ctx context.Context, msgs []llm.Message, opts llm.CompletionOpts) (<-chan llm.StreamEvent, error) {
	pk := &peek{}
	in, err := c.Provider.StreamCompletion(context.WithValue(ctx, peekKey{}, pk), msgs, opts)
	if err != nil {
		return nil, err
	}
	out := make(chan llm.StreamEvent)
	go func() {
		defer close(out)
		said := false
		for ev := range in {
			if ev.Token != "" || ev.Thinking != "" || len(ev.ToolCalls) > 0 || ev.ToolCallDelta != nil {
				said = true
			}
			if ev.Done && ev.Err == nil && !said {
				if body := pk.text(); body != "" && !strings.Contains(body, `"choices"`) {
					ev = llm.StreamEvent{Done: true, Err: c.notACompletion(body)}
				}
			}
			select {
			case out <- ev:
			case <-ctx.Done():
				for range in {
				}
				return
			}
		}
	}()
	return out, nil
}

// notACompletion words what the service sent: its name, then the first
// eighty characters of the body with any key-shaped token removed.
func (c checked) notACompletion(body string) error {
	quote := []rune(strings.Join(strings.Fields(c.scrub(body)), " "))
	tail := ""
	if len(quote) > quoteChars {
		quote, tail = quote[:quoteChars], "..."
	}
	return fmt.Errorf("%s answered, but not with a reply (it sent %s%s). Check the address and the model name", c.name, string(quote), tail)
}
