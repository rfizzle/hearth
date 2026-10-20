package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rfizzle/llm"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

const gatewayKey = "gw-secret-0123456789-tail"

// fakeCompat is an OpenAI-compatible service in memory: it streams the
// tokens it is given and remembers what it was asked.
type fakeCompat struct {
	tokens []string
	mu     sync.Mutex
	host   string
	path   string
	auth   string
	model  string
}

// client reaches the fake whatever host a request names, so nothing leaves
// the process.
func (f *fakeCompat) client() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		f.ServeHTTP(rec, r)
		return rec.Result(), nil
	})}
}

func (f *fakeCompat) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	f.host, f.path, f.auth, f.model = r.URL.Host, r.URL.Path, r.Header.Get("Authorization"), body.Model
	f.mu.Unlock()
	if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	for i, tok := range f.tokens {
		chunk := map[string]any{"id": "c1", "object": "chat.completion.chunk", "created": 1, "model": body.Model,
			"choices": []map[string]any{{"index": 0, "delta": map[string]string{"content": tok}}}}
		if i == len(f.tokens)-1 {
			chunk["choices"].([]map[string]any)[0]["finish_reason"] = "stop"
		}
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func TestCustomProfileResolvesAndStreams(t *testing.T) {
	Init()
	dir := filepath.Join(t.TempDir(), "Hearth")
	fake := &fakeCompat{tokens: []string{"hel", "lo"}}
	accts, err := Open(dir, fake.client())
	if err != nil {
		t.Fatal(err)
	}
	var seen []Account
	accts.OnChange = func(a Account) { seen = append(seen, a) }

	// A built-in key is there too; the added service is still preferred.
	if err := accts.SetKey(Anthropic, "sk-ant-abcdef1234567890"); err != nil {
		t.Fatal(err)
	}
	acct, err := accts.SetProfile(Profile{Name: " Acme gateway ", BaseURL: "https://llm.acme.example/v1/", DefaultModel: "acme-large"}, " "+gatewayKey+"\n")
	if err != nil {
		t.Fatalf("SetProfile: %v", err)
	}
	want := Account{Provider: Compatible, State: SignedIn, Label: "tail", Profile: acct.Profile, DisplayName: "Acme gateway", BaseURL: "https://llm.acme.example/v1", DefaultModel: "acme-large"}
	if acct.Profile == "" || acct != want {
		t.Fatalf("SetProfile = %+v, want %+v", acct, want)
	}
	if last := seen[len(seen)-1]; last != want {
		t.Errorf("OnChange saw %+v", last)
	}

	// The key and the settings are owner-only files beside each other, and
	// the settings hold no key.
	keyDir := accts.Keys.(FileKeys).Dir
	for _, name := range []string{"compat-" + acct.Profile, "compat-" + acct.Profile + "-profile"} {
		info, err := os.Stat(filepath.Join(keyDir, name))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("%s: %v, %v; want 0600", name, info, err)
		}
	}
	meta, _ := os.ReadFile(filepath.Join(keyDir, "compat-"+acct.Profile+"-profile"))
	if strings.Contains(string(meta), gatewayKey) || !strings.Contains(string(meta), "acme-large") {
		t.Errorf("settings file = %s", meta)
	}

	// A relaunch lists it after the built-ins and resolves to it at its model.
	again, err := Open(dir, fake.client())
	if err != nil {
		t.Fatal(err)
	}
	list := again.List()
	if len(list) != len(Providers)+1 || list[len(list)-1] != want {
		t.Fatalf("List after relaunch = %+v", list)
	}
	for _, tier := range []Tier{Fast, Balanced, Best} {
		m, err := again.Resolve(tier)
		if err != nil || m.Account != Compatible || m.Profile != acct.Profile || m.Name != "acme-large" || m.Provider.Name() != "Acme gateway" {
			t.Fatalf("Resolve(%d) = %+v, %v", tier, m, err)
		}
	}

	m, _ := again.Resolve(Balanced)
	ch, err := StreamFunc(m)(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "hi"}}, nil, "")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	var out bytes.Buffer
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("stream event: %v", ev.Err)
		}
		out.WriteString(ev.Token)
	}
	if out.String() != "hello" {
		t.Errorf("streamed %q, want hello", out.String())
	}
	if fake.host != "llm.acme.example" || fake.path != "/v1/chat/completions" || fake.auth != "Bearer "+gatewayKey || fake.model != "acme-large" {
		t.Errorf("the service saw %s %s, auth %q, model %q", fake.host, fake.path, fake.auth, fake.model)
	}

	// The key is scrubbed like any other.
	scrubbed := again.Scrub()(llm.Message{Role: llm.RoleTool, Content: "it is " + gatewayKey})
	if strings.Contains(scrubbed.Content, gatewayKey) {
		t.Errorf("the gateway key reached the message: %q", scrubbed.Content)
	}

	// Removing it leaves the built-in to answer and publishes it signed out.
	if err := accts.ClearProfile(acct.Profile); err != nil {
		t.Fatal(err)
	}
	if last := seen[len(seen)-1]; last.Profile != acct.Profile || last.State != SignedOut {
		t.Errorf("OnChange after clear saw %+v", last)
	}
	if len(accts.List()) != len(Providers) {
		t.Errorf("List after clear = %+v", accts.List())
	}
	if m, err := accts.Resolve(Balanced); err != nil || m.Account != Anthropic {
		t.Errorf("Resolve after clear = %+v, %v; want Anthropic", m, err)
	}
	if err := accts.ClearProfile(acct.Profile); err == nil {
		t.Error("a second ClearProfile succeeded")
	}
}

func TestSetProfileChecksWhatItIsGiven(t *testing.T) {
	accts := &Accounts{Keys: &memKeys{}}
	tests := []struct {
		name string
		p    Profile
		key  string
		ok   bool
	}{
		{"complete", Profile{Name: "G", BaseURL: "https://g.example/v1", DefaultModel: "m"}, gatewayKey, true},
		{"local takes no key", Profile{Name: "Ollama", BaseURL: "http://localhost:11434/v1", DefaultModel: "llama3.2"}, "", true},
		{"loopback address takes no key", Profile{Name: "LM", BaseURL: "http://127.0.0.1:1234/v1", DefaultModel: "m"}, "", true},
		{"no key", Profile{Name: "G", BaseURL: "https://g.example/v1", DefaultModel: "m"}, " ", false},
		{"no name", Profile{BaseURL: "https://g.example/v1", DefaultModel: "m"}, gatewayKey, false},
		{"no model", Profile{Name: "G", BaseURL: "https://g.example/v1"}, gatewayKey, false},
		{"no address", Profile{Name: "G", DefaultModel: "m"}, gatewayKey, false},
		{"not a web address", Profile{Name: "G", BaseURL: "file:///etc/passwd", DefaultModel: "m"}, gatewayKey, false},
		{"plain http elsewhere", Profile{Name: "G", BaseURL: "http://g.example/v1", DefaultModel: "m"}, gatewayKey, false},
		{"key in the address", Profile{Name: "G", BaseURL: "https://u:" + gatewayKey + "@g.example/v1", DefaultModel: "m"}, gatewayKey, false},
		{"key in the query", Profile{Name: "G", BaseURL: "https://g.example/v1?api_key=" + gatewayKey, DefaultModel: "m"}, gatewayKey, false},
		{"key in the fragment", Profile{Name: "G", BaseURL: "https://g.example/v1#" + gatewayKey, DefaultModel: "m"}, gatewayKey, false},
		{"unknown id", Profile{ID: "deadbeef", Name: "G", BaseURL: "https://g.example/v1", DefaultModel: "m"}, gatewayKey, false},
		{"an id that is a path", Profile{ID: "../keys/anthropic", Name: "G", BaseURL: "https://g.example/v1", DefaultModel: "m"}, gatewayKey, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := accts.SetProfile(tt.p, tt.key)
			if (err == nil) != tt.ok {
				t.Fatalf("SetProfile = %v, want ok %v", err, tt.ok)
			}
			if err != nil && strings.Contains(err.Error(), gatewayKey) {
				t.Errorf("the error carries the key: %v", err)
			}
		})
	}
}

func TestSetProfileChangesInPlaceAndKeepsTheKey(t *testing.T) {
	accts := &Accounts{Keys: &memKeys{}}
	first, err := accts.SetProfile(Profile{Name: "First", BaseURL: "https://a.example/v1", DefaultModel: "m"}, gatewayKey)
	if err != nil {
		t.Fatal(err)
	}
	second, err := accts.SetProfile(Profile{Name: "Second", BaseURL: "https://b.example/v1", DefaultModel: "m"}, gatewayKey+"2")
	if err != nil {
		t.Fatal(err)
	}
	changed, err := accts.SetProfile(Profile{ID: first.Profile, Name: "First, renamed", BaseURL: "https://a.example/v2", DefaultModel: "m2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if changed.Profile != first.Profile || changed.Label != "tail" || changed.DefaultModel != "m2" {
		t.Errorf("changed = %+v", changed)
	}
	// The first added stays first after a change.
	got := accts.Profiles()
	if len(got) != 2 || got[0].ID != first.Profile || got[0].Name != "First, renamed" || got[1].ID != second.Profile {
		t.Errorf("Profiles = %+v", got)
	}
}

// TestKnownServicesAgreeWithTheContract fails when a row lacks an address
// or a help page, an id repeats, a builtin row names no provider, or the
// table and the contract disagree about the kinds.
func TestKnownServicesAgreeWithTheContract(t *testing.T) {
	kinds := map[string]bool{}
	for name, v := range hearthv1.KnownServiceKind_value {
		if v != 0 {
			kinds[name] = false
		}
	}
	ids := map[string]bool{}
	builtins := 0
	for _, s := range KnownServices() {
		if s.ID == "" || s.Name == "" || s.BaseURL == "" || s.HelpURL == "" {
			t.Errorf("%+v lacks an id, a name, an address or a help page", s)
		}
		if _, _, err := checkBaseURL(s.BaseURL); err != nil {
			t.Errorf("%s: address %q is refused: %v", s.ID, s.BaseURL, err)
		}
		if ids[s.ID] {
			t.Errorf("id %q repeats", s.ID)
		}
		ids[s.ID] = true
		k := "KNOWN_SERVICE_KIND_" + strings.ToUpper(string(s.Kind))
		if _, ok := kinds[k]; !ok {
			t.Errorf("%s has kind %q, the contract has no %s", s.ID, s.Kind, k)
		}
		kinds[k] = true
		switch {
		case s.Kind == Builtin:
			builtins++
			if !known(Provider(s.ID)) {
				t.Errorf("builtin row %q is not a provider", s.ID)
			}
		case s.DefaultModel == "":
			t.Errorf("%s offers no default model", s.ID)
		}
	}
	if builtins != len(Providers) {
		t.Errorf("%d builtin rows, want one per provider (%d)", builtins, len(Providers))
	}
	for k, used := range kinds {
		if !used {
			t.Errorf("the contract has %s, no row uses it", k)
		}
	}
}

// settingsFail is a key store that refuses to keep a service's settings.
type settingsFail struct{ memKeys }

func (s *settingsFail) Save(n, v string) error {
	if isProfileMeta(n) {
		return errors.New("disk full")
	}
	return s.memKeys.Save(n, v)
}

func TestSetProfileLeavesNothingWhenTheSettingsCannotBeSaved(t *testing.T) {
	keys := &settingsFail{}
	accts := &Accounts{Keys: keys}
	if _, err := accts.SetProfile(Profile{Name: "G", BaseURL: "https://g.example/v1", DefaultModel: "m"}, gatewayKey); err == nil {
		t.Fatal("SetProfile succeeded with the settings refused")
	}
	if names, _ := keys.Names(); len(names) != 0 {
		t.Errorf("the failed add left %v behind", names)
	}
	if got := accts.Profiles(); len(got) != 0 {
		t.Errorf("Profiles = %+v, want none", got)
	}
}
