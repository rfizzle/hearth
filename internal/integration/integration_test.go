package integration

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"

	"github.com/rfizzle/hearth/internal/provider"
)

const testKey = "BSA-secret-0123456789-wxyz"

func newIntegrations(t *testing.T) (*Integrations, *[]Account) {
	t.Helper()
	var seen []Account
	i := &Integrations{
		Keys:     provider.FileKeys{Dir: filepath.Join(t.TempDir(), "keys")},
		OnChange: func(a Account) { seen = append(seen, a) },
	}
	return i, &seen
}

func brave(t *testing.T, i *Integrations) Account {
	t.Helper()
	for _, a := range i.List() {
		if a.Name == BraveSearch {
			return a
		}
	}
	t.Fatal("List has no Brave Search")
	return Account{}
}

func TestNoKeyMeansNoSearcher(t *testing.T) {
	i, seen := newIntegrations(t)
	a := brave(t, i)
	if a.Configured || a.Label != "" {
		t.Errorf("fresh account = %+v, want not configured", a)
	}
	if a.Kind != WebSearch || a.Title != "Brave Search" || a.HelpURL == "" {
		t.Errorf("account = %+v", a)
	}
	if i.Searcher() != nil {
		t.Error("Searcher is non-nil with no key")
	}
	if len(*seen) != 0 {
		t.Errorf("published %d changes before any change", len(*seen))
	}
}

func TestSetKeySwitchesSearchOnAndClearReverses(t *testing.T) {
	i, seen := newIntegrations(t)
	// Wired before the key exists; the key set afterwards must take effect.
	got, err := i.SetKey(BraveSearch, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Configured || got.Label != "wxyz" {
		t.Errorf("SetKey returned %+v", got)
	}
	if a := brave(t, i); !a.Configured || a.Label != "wxyz" {
		t.Errorf("List after SetKey = %+v", a)
	}
	if i.Searcher() == nil {
		t.Error("Searcher is nil with a key")
	}
	if len(*seen) != 1 || !(*seen)[0].Configured {
		t.Errorf("changes after SetKey = %+v", *seen)
	}

	got, err = i.Clear(BraveSearch)
	if err != nil {
		t.Fatal(err)
	}
	if got.Configured || got.Label != "" {
		t.Errorf("Clear returned %+v", got)
	}
	if a := brave(t, i); a.Configured || a.Label != "" {
		t.Errorf("List after Clear = %+v", a)
	}
	if i.Searcher() != nil {
		t.Error("Searcher is non-nil after Clear")
	}
	if len(*seen) != 2 || (*seen)[1].Configured {
		t.Errorf("changes after Clear = %+v", *seen)
	}
}

func TestRefusals(t *testing.T) {
	i, seen := newIntegrations(t)
	if _, err := i.SetKey("nope", testKey); !errors.Is(err, ErrUnknown) {
		t.Errorf("SetKey of an unknown name = %v", err)
	}
	if _, err := i.Clear("nope"); !errors.Is(err, ErrUnknown) {
		t.Errorf("Clear of an unknown name = %v", err)
	}
	if _, err := i.SetKey("../anthropic", testKey); !errors.Is(err, ErrUnknown) {
		t.Errorf("SetKey of a path = %v", err)
	}
	if _, err := i.SetKey(BraveSearch, ""); !errors.Is(err, ErrEmptyKey) {
		t.Errorf("SetKey of an empty key = %v", err)
	}
	if names, _ := i.Keys.Names(); len(names) != 0 {
		t.Errorf("a refused call stored %v", names)
	}
	if len(*seen) != 0 {
		t.Errorf("a refused call published %+v", *seen)
	}
}

func TestShortKeyHasNoLabel(t *testing.T) {
	i, _ := newIntegrations(t)
	a, err := i.SetKey(BraveSearch, "short")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Configured || a.Label != "" {
		t.Errorf("short key account = %+v", a)
	}
}

// failingKeys is a store whose writes fail.
type failingKeys struct{ provider.KeyStore }

func (failingKeys) Save(string, string) error { return errors.New("disk full") }

func TestKeyNeverAppearsInOutputs(t *testing.T) {
	i, seen := newIntegrations(t)
	if _, err := i.SetKey(BraveSearch, testKey); err != nil {
		t.Fatal(err)
	}
	for _, a := range append(i.List(), *seen...) {
		if strings.Contains(string(a.Name)+string(a.Kind)+a.Title+a.Label+a.HelpURL, testKey) {
			t.Errorf("the key reached %+v", a)
		}
	}
	_, err := (&Integrations{Keys: failingKeys{i.Keys}}).SetKey(BraveSearch, testKey)
	if err == nil || strings.Contains(err.Error(), testKey) {
		t.Errorf("a failed save = %v, want an error without the key", err)
	}
}

func TestStoredKeyIsScrubbed(t *testing.T) {
	// The same store the provider accounts hold, as hearthd wires it.
	accts, err := provider.Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	i := &Integrations{Keys: accts.Keys}
	if _, err := i.SetKey(BraveSearch, testKey); err != nil {
		t.Fatal(err)
	}
	scrub := accts.Scrub()
	got := scrub(llm.Message{Role: llm.RoleTool, Content: "result " + testKey + " end"})
	if strings.Contains(got.Content, testKey) {
		t.Errorf("the key survived the scrub: %q", got.Content)
	}
}

func TestSetKeyFailureIsReported(t *testing.T) {
	i, seen := newIntegrations(t)
	i.Keys = failingKeys{i.Keys}
	if _, err := i.SetKey(BraveSearch, testKey); err == nil {
		t.Fatal("a failed save was not reported")
	}
	if len(*seen) != 0 {
		t.Errorf("a failed save published %+v", *seen)
	}
}

// roundTrip is a fake transport.
type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSearcherUsesTheStoredKey(t *testing.T) {
	var token string
	i, _ := newIntegrations(t)
	i.Client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		token = r.Header.Get("X-Subscription-Token")
		return &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"web":{"results":[]}}`)),
			Request:    r,
		}, nil
	})}
	if _, err := i.SetKey(BraveSearch, testKey); err != nil {
		t.Fatal(err)
	}
	s := i.Searcher()
	if s == nil {
		t.Fatal("no searcher")
	}
	if _, err := s.Search(context.Background(), "hearth", 3); err != nil {
		t.Fatal(err)
	}
	if token != testKey {
		t.Errorf("request carried token %q", token)
	}
}

// TestTableAgreesWithTheContract fails when the contract names a service or a
// kind this table lacks, or the table names one the contract lacks.
func TestTableAgreesWithTheContract(t *testing.T) {
	enum := func(prefix string, n string) string {
		return prefix + strings.ToUpper(strings.ReplaceAll(n, "-", "_"))
	}
	integrations := map[string]bool{}
	for name, v := range hearthv1.Integration_value {
		if v != 0 {
			integrations[name] = false
		}
	}
	kinds := map[string]bool{}
	for name, v := range hearthv1.IntegrationKind_value {
		if v != 0 {
			kinds[name] = false
		}
	}
	for _, s := range table {
		n := enum("INTEGRATION_", string(s.Name))
		if _, ok := integrations[n]; !ok {
			t.Errorf("table has %q, the contract has no %s", s.Name, n)
		}
		integrations[n] = true
		k := enum("INTEGRATION_KIND_", string(s.Kind))
		if _, ok := kinds[k]; !ok {
			t.Errorf("table has kind %q, the contract has no %s", s.Kind, k)
		}
		kinds[k] = true
		if s.Kind == WebSearch && s.Search == nil {
			t.Errorf("%q is a search service with no backend", s.Name)
		}
		if s.Title == "" || s.HelpURL == "" {
			t.Errorf("%q has no title or help address", s.Name)
		}
	}
	for n, used := range integrations {
		if !used {
			t.Errorf("the contract has %s, the table has no row for it", n)
		}
	}
	for n, used := range kinds {
		if !used {
			t.Errorf("the contract has %s, no table row uses it", n)
		}
	}
}
