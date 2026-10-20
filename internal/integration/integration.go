package integration

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/hearth/internal/tools/web"
)

// Name identifies an integration. It is also the name its key is stored under.
type Name string

// BraveSearch is the Brave Search API.
const BraveSearch Name = "brave-search"

// Kind is what an integration provides, so the app can say which tool a key
// switches on.
type Kind string

// WebSearch is a service that backs web.search.
const WebSearch Kind = "web-search"

// Account is what the core says about one integration. It never holds the key.
type Account struct {
	Name       Name
	Kind       Kind
	Title      string
	Configured bool
	// Label is the key's last four characters.
	Label   string
	HelpURL string
}

// ErrUnknown is what a call about a name outside the table returns.
var ErrUnknown = errors.New("that service is not one hearth knows")

// ErrEmptyKey is what SetKey returns for a blank key.
var ErrEmptyKey = errors.New("paste the key first; it was empty")

// spec is one row of the table: what a service is and how to reach it.
type spec struct {
	Name    Name
	Kind    Kind
	Title   string
	HelpURL string
	// Search builds the backend for a stored key; set for WebSearch services.
	Search func(key string, client *http.Client) web.Searcher
}

// table is every service hearth knows. Another search service is one
// constant above and one row here, beside a backend file in tools/web.
var table = []spec{
	{
		Name:    BraveSearch,
		Kind:    WebSearch,
		Title:   "Brave Search",
		HelpURL: "https://brave.com/search/api/",
		Search: func(key string, c *http.Client) web.Searcher {
			return web.NewBrave(key, c)
		},
	},
}

func find(name Name) (spec, bool) {
	for _, s := range table {
		if s.Name == name {
			return s, true
		}
	}
	return spec{}, false
}

// Integrations is the person's integrations over one key store.
type Integrations struct {
	// Keys is the store shared with provider.Accounts.
	Keys keystore.Store
	// Client reaches the backends; nil is a plain client.
	Client *http.Client
	// OnChange is told each time an account changes. Set it before the first
	// call; it runs on the calling goroutine and must not block.
	OnChange func(Account)

	mu sync.Mutex
}

// List returns one account per known integration, in table order.
func (i *Integrations) List() []Account {
	out := make([]Account, 0, len(table))
	for _, s := range table {
		out = append(out, i.account(s))
	}
	return out
}

func (i *Integrations) account(s spec) Account {
	a := Account{Name: s.Name, Kind: s.Kind, Title: s.Title, HelpURL: s.HelpURL}
	key, err := i.Keys.Load(string(s.Name))
	if err == nil && key != "" {
		a.Configured = true
		a.Label = lastFour(key)
	}
	return a
}

// lastFour is the label for a key. A short key would give away too much of
// itself, so it has none.
func lastFour(key string) string {
	if len(key) < 12 {
		return ""
	}
	return key[len(key)-4:]
}

// SetKey stores the key for name and publishes the change.
func (i *Integrations) SetKey(name Name, key string) (Account, error) {
	s, ok := find(name)
	if !ok {
		return Account{}, ErrUnknown
	}
	if key == "" {
		return Account{}, ErrEmptyKey
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.Keys.Save(string(name), key); err != nil {
		return Account{}, fmt.Errorf("keep the %s key: %w", s.Title, err)
	}
	return i.publish(s), nil
}

// Clear removes the key for name and publishes the change.
func (i *Integrations) Clear(name Name) (Account, error) {
	s, ok := find(name)
	if !ok {
		return Account{}, ErrUnknown
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if err := i.Keys.Clear(string(name)); err != nil {
		return Account{}, fmt.Errorf("remove the %s key: %w", s.Title, err)
	}
	return i.publish(s), nil
}

func (i *Integrations) publish(s spec) Account {
	a := i.account(s)
	if i.OnChange != nil {
		i.OnChange(a)
	}
	return a
}

// Searcher builds the backend of the first web-search service that holds a
// key, or returns nil when none does, so web.search is not offered. It is
// built fresh on each call.
// See docs/decisions.md#hearth--decisions
func (i *Integrations) Searcher() web.Searcher {
	for _, s := range table {
		if s.Kind != WebSearch || s.Search == nil {
			continue
		}
		key, err := i.Keys.Load(string(s.Name))
		if err != nil || key == "" {
			continue
		}
		return s.Search(key, i.Client)
	}
	return nil
}
