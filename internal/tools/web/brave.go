package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	braveEndpoint = "https://api.search.brave.com/res/v1/web/search"
	braveTimeout  = 15 * time.Second
	braveMaxBody  = 1 << 20
)

// Brave is the Brave Search API's web search, as a Searcher.
type Brave struct {
	key    string
	client *http.Client
}

// NewBrave is Brave Search asked with key through client; a nil client is a
// plain one. The key travels in a request header and nowhere else.
func NewBrave(key string, client *http.Client) *Brave {
	c := http.Client{}
	if client != nil {
		c = *client
	}
	// A redirect is answered as a refusal rather than followed: the client
	// would send the key's header on to wherever it pointed.
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Brave{key: key, client: &c}
}

// braveResponse is the part of Brave's web search response a hit is made of.
type braveResponse struct {
	Web struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
		} `json:"results"`
	} `json:"web"`
}

// braveError is the body Brave answers a refused request with.
type braveError struct {
	Error struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
	} `json:"error"`
}

// Search asks Brave for count hits for query, within 15 seconds.
func (b *Brave) Search(ctx context.Context, query string, count int) ([]Hit, error) {
	ctx, cancel := context.WithTimeout(ctx, braveTimeout)
	defer cancel()
	q := url.Values{"q": {query}, "count": {strconv.Itoa(count)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, braveEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", b.key)
	resp, err := b.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("no answer from Brave Search within 15 seconds; try again")
		}
		return nil, errors.New("could not reach Brave Search; check the connection and try again")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, braveMaxBody))
	if err != nil {
		return nil, errors.New("the answer from Brave Search stopped part way; try again")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, braveRefusal(resp.StatusCode, body)
	}
	var parsed braveResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, errors.New("the answer from Brave Search is not a search result")
	}
	hits := make([]Hit, 0, len(parsed.Web.Results))
	for _, r := range parsed.Web.Results {
		hits = append(hits, Hit{Title: reduceHTML([]byte(r.Title)), URL: r.URL, Snippet: reduceHTML([]byte(r.Description))})
	}
	return hits, nil
}

// braveRefusal is Brave's error answer in plain words. Brave refuses a bad
// key with 401, 403 or a 422 whose code names the token.
func braveRefusal(status int, body []byte) error {
	var e braveError
	_ = json.Unmarshal(body, &e) // a body that is not Brave's error shape leaves the status alone to go on
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden || strings.Contains(e.Error.Code, "TOKEN"):
		return errors.New("the key was refused by Brave Search; the owner needs to check the Brave Search key")
	case status == http.StatusTooManyRequests:
		return errors.New("the search limit for this key was reached at Brave Search; wait a while before searching again")
	case status >= 500:
		return fmt.Errorf("trouble at Brave Search (status %d); try again later", status)
	case e.Error.Detail != "":
		return fmt.Errorf("the search was refused by Brave Search: %s", e.Error.Detail)
	}
	return fmt.Errorf("the search was refused by Brave Search (status %d)", status)
}
