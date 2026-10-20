package models_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/models"
	"github.com/rfizzle/llm"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

// runtimes is the network of a Mac: the hosts it names answer a model list,
// every other address refuses. It records every address asked.
type runtimes struct {
	mu     sync.Mutex
	lists  map[string]string
	asked  []string
	client *http.Client
}

func newRuntimes(lists map[string]string) *runtimes {
	r := &runtimes{lists: lists}
	r.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		r.mu.Lock()
		r.asked = append(r.asked, req.URL.Host+req.URL.Path)
		body, ok := r.lists[req.URL.Host]
		r.mu.Unlock()
		if !ok {
			return nil, errors.New("connection refused")
		}
		rec := httptest.NewRecorder()
		rec.Header().Set("Content-Type", "application/json")
		_, _ = rec.WriteString(body)
		return rec.Result(), nil
	})}
	return r
}

const twoModels = `{"data":[{"id":"llama3.2"},{"id":"qwen2.5-coder"}]}`

func open(t *testing.T, c *http.Client) *provider.Accounts {
	t.Helper()
	provider.Init()
	accts, err := provider.Open(filepath.Join(t.TempDir(), "Hearth"), c)
	if err != nil {
		t.Fatal(err)
	}
	return accts
}

// signedIn is an account with an Anthropic key and a local service.
func signedIn(t *testing.T, c *http.Client) (*provider.Accounts, provider.Account) {
	t.Helper()
	accts := open(t, c)
	if err := accts.SetKey(provider.Anthropic, "sk-ant-api03-abcdef1234567890"); err != nil {
		t.Fatal(err)
	}
	local, err := accts.SetProfile(provider.Profile{Name: "Ollama", BaseURL: "http://localhost:11434/v1", DefaultModel: "llama3.2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return accts, local
}

func TestOverrideBeatsTheMappingBeatsTheDefaults(t *testing.T) {
	accts, local := signedIn(t, nil)
	mapping := []models.Row{{Tier: provider.Fast, Pick: provider.Pick{Provider: provider.Anthropic, Model: "mapped-model"}}}
	pinned := models.Encode(provider.Pick{Provider: provider.Compatible, Profile: local.Profile, Model: "qwen2.5-coder"})

	m, err := models.Resolve(accts, models.Choice{Tier: provider.Fast, Override: pinned}, mapping)
	if err != nil || m.Account != provider.Compatible || m.Profile != local.Profile || m.Name != "qwen2.5-coder" {
		t.Errorf("override = %+v, %v; want qwen2.5-coder on the local service", m, err)
	}
	m, err = models.Resolve(accts, models.Choice{Tier: provider.Fast}, mapping)
	if err != nil || m.Account != provider.Anthropic || m.Name != "mapped-model" {
		t.Errorf("mapping = %+v, %v; want mapped-model on anthropic", m, err)
	}
	// A tier with no row follows the defaults: the added service first.
	m, err = models.Resolve(accts, models.Choice{Tier: provider.Best}, mapping)
	if err != nil || m.Account != provider.Compatible || m.Name != "llama3.2" {
		t.Errorf("defaults = %+v, %v; want the added service's model", m, err)
	}
}

func TestAMissingProviderFallsBackWithAPlainError(t *testing.T) {
	accts := open(t, nil)
	if err := accts.SetKey(provider.Anthropic, "sk-ant-api03-abcdef1234567890"); err != nil {
		t.Fatal(err)
	}
	gone := provider.Pick{Provider: provider.Gemini, Model: "gemini-x"}
	mapping := []models.Row{{Tier: provider.Balanced, Pick: gone}}

	m, err := models.Resolve(accts, models.Choice{Tier: provider.Balanced}, mapping)
	if err != nil || m.Account != provider.Anthropic || m.Name != llm.Defaults("anthropic").Model {
		t.Errorf("a mapped provider that is signed out gave %+v, %v; want the defaults", m, err)
	}
	// A pinned model is never swapped for another.
	_, err = models.Resolve(accts, models.Choice{Tier: provider.Balanced, Override: models.Encode(gone)}, mapping)
	if err == nil || !strings.Contains(err.Error(), "gemini is not signed in") {
		t.Errorf("a pinned model on a signed-out provider = %v, want a plain error", err)
	}
	_, err = models.Resolve(accts, models.Choice{Tier: provider.Balanced, Override: "garbage"}, nil)
	if err == nil || !strings.Contains(err.Error(), "not one hearth can read") {
		t.Errorf("an unreadable override = %v", err)
	}
	_, err = models.Resolve(open(t, nil), models.Choice{Tier: provider.Fast}, mapping)
	if !errors.Is(err, provider.ErrNoAccount) {
		t.Errorf("nothing signed in = %v, want no account", err)
	}
	// A removed service is named plainly too.
	_, err = models.Resolve(accts, models.Choice{Tier: provider.Fast, Override: "openai-compatible:deadbeef:m"}, nil)
	if err == nil || !strings.Contains(err.Error(), "was removed") {
		t.Errorf("a removed service = %v", err)
	}
}

func TestEncodeKeepsAModelsOwnColons(t *testing.T) {
	p := provider.Pick{Provider: provider.Compatible, Profile: "ab12cd34", Model: "llama3:8b"}
	got, ok := models.Parse(models.Encode(p))
	if !ok || got != p {
		t.Errorf("Parse(Encode(%+v)) = %+v, %v", p, got, ok)
	}
}

func TestDetectMarksTheRuntimeThatAnswers(t *testing.T) {
	net := newRuntimes(map[string]string{"localhost:11434": twoModels})
	accts := open(t, net.client)

	var ollama, lmstudio models.Service
	for _, s := range models.Detect(t.Context(), accts) {
		switch s.ID {
		case "ollama":
			ollama = s
		case "lmstudio":
			lmstudio = s
		default:
			if s.Detected || s.Models != nil {
				t.Errorf("%s was probed", s.ID)
			}
		}
	}
	if !ollama.Detected || !slices.Equal(ollama.Models, []string{"llama3.2", "qwen2.5-coder"}) {
		t.Errorf("ollama = %+v, want detected with two models", ollama)
	}
	if lmstudio.ID == "" || lmstudio.Detected || lmstudio.Models != nil {
		t.Errorf("lmstudio = %+v, want listed and not detected", lmstudio)
	}
	want := []string{"localhost:11434/v1/models", "localhost:1234/v1/models"}
	if !slices.Equal(net.asked, want) {
		t.Errorf("asked %q, want only %q", net.asked, want)
	}
}

func TestOptionsListWhatTheAccountsOffer(t *testing.T) {
	net := newRuntimes(map[string]string{"localhost:11434": twoModels})
	accts, local := signedIn(t, net.client)
	d := llm.Defaults("anthropic")

	got := models.Options(accts, models.Detect(t.Context(), accts))
	var names []string
	for _, o := range got {
		names = append(names, o.Label+":"+o.Pick.Model)
	}
	want := []string{"Anthropic:" + d.Model, "Anthropic:" + d.CheapModel, "Ollama:llama3.2", "Ollama:qwen2.5-coder"}
	if !slices.Equal(names, want) {
		t.Errorf("options = %q, want %q", names, want)
	}
	if last := got[len(got)-1].Pick; last.Profile != local.Profile || last.Provider != provider.Compatible {
		t.Errorf("a detected model is picked on the added service, got %+v", last)
	}
}
