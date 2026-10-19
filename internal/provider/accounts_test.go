package provider

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"
)

// recorder collects what OnChange publishes.
type recorder struct {
	ch chan Account
}

func newRecorder() *recorder { return &recorder{ch: make(chan Account, 32)} }

func (r *recorder) record(a Account) {
	if a.Provider == ChatGPT {
		r.ch <- a
	}
}

// next waits for the next ChatGPT change.
func (r *recorder) next(t *testing.T) Account {
	t.Helper()
	select {
	case a := <-r.ch:
		return a
	case <-time.After(10 * time.Second):
		t.Fatal("no account change was published")
		return Account{}
	}
}

// rig is accounts over a temp dir with the fake authorization server and an
// in-memory loopback listener.
type rig struct {
	accts *Accounts
	auth  *fakeAuth
	l     *memListener
	rec   *recorder
}

func newRig(t *testing.T) *rig {
	t.Helper()
	Init()
	auth := newFakeAuth(t)
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), auth.client())
	if err != nil {
		t.Fatal(err)
	}
	l := newMemListener()
	accts.login = chatgpt.LoginOptions{
		Listen: func() (net.Listener, error) { return l, nil },
		Wait:   10 * time.Second,
	}
	rec := newRecorder()
	accts.OnChange = rec.record
	return &rig{accts: accts, auth: auth, l: l, rec: rec}
}

func TestStartLoginSignsIn(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()

	page, err := r.accts.StartLogin(ctx)
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	if !strings.HasPrefix(page, chatgpt.AuthorizeURL+"?") {
		t.Fatalf("page = %q, want the authorize page", page)
	}
	if got := r.rec.next(t); got.State != SigningIn {
		t.Fatalf("first change = %+v, want SigningIn", got)
	}
	if got := r.accts.List()[0]; got.State != SigningIn {
		t.Errorf("List while waiting = %+v, want SigningIn", got)
	}

	// A second call while the first waits is the same page and starts nothing.
	again, err := r.accts.StartLogin(ctx)
	if err != nil || again != page {
		t.Fatalf("second StartLogin = %q, %v; want %q", again, err, page)
	}

	// The person comes back from the browser.
	if err := r.auth.browse(r.l)(page); err != nil {
		t.Fatal(err)
	}
	got := r.rec.next(t)
	if got.State != SignedIn || got.Label != "person@example.com" || got.Failure != "" {
		t.Errorf("final change = %+v, want SignedIn as the person", got)
	}
	if _, err := r.accts.Resolve(Balanced); err != nil {
		t.Errorf("Resolve after sign-in: %v", err)
	}

	// With the slot free, a later sign-in starts a new attempt.
	if _, started := r.accts.begin(); !started {
		t.Error("the sign-in slot was not released")
	}
}

func TestStartLoginReturnsBeforeTheBrowserComesBack(t *testing.T) {
	r := newRig(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := r.accts.StartLogin(context.Background()); err != nil {
			t.Errorf("StartLogin: %v", err)
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("StartLogin did not return while the sign-in was waiting")
	}
}

func TestStartLoginFailureIsPublished(t *testing.T) {
	r := newRig(t)
	page, err := r.accts.StartLogin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r.rec.next(t) // SigningIn

	// The person declines in the browser.
	u, err := url.Parse(page)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	back := url.Values{"state": {q.Get("state")}, "error": {"access_denied"}}
	c := http.Client{Transport: &http.Transport{DialContext: r.l.dial}}
	resp, err := c.Get(q.Get("redirect_uri") + "?" + back.Encode())
	if err == nil {
		_ = resp.Body.Close()
	}

	got := r.rec.next(t)
	if got.State != SignedOut || !strings.Contains(got.Failure, "declined") {
		t.Errorf("change = %+v, want SignedOut with the failure in words", got)
	}
	if list := r.accts.List()[0]; list.State != SignedOut || list.Failure != got.Failure {
		t.Errorf("List = %+v, want the failure kept", list)
	}
}

func TestStartLoginFailureBeforeThePage(t *testing.T) {
	r := newRig(t)
	r.accts.login.Listen = func() (net.Listener, error) { return nil, errors.New("no port") }
	if _, err := r.accts.StartLogin(context.Background()); err == nil {
		t.Fatal("StartLogin succeeded without a listener")
	}
	var last Account
	for i := 0; i < 2; i++ {
		last = r.rec.next(t)
	}
	if last.State != SignedOut || last.Failure == "" {
		t.Errorf("last change = %+v, want SignedOut with a failure", last)
	}
}

func TestStartLoginHonoursTheCallersContext(t *testing.T) {
	r := newRig(t)
	r.accts.login.Listen = func() (net.Listener, error) {
		time.Sleep(200 * time.Millisecond)
		return r.l, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.accts.StartLogin(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("StartLogin = %v, want canceled", err)
	}
	r.l.Close()
}

// memKeys is a KeyStore in memory, the seam a test or the Keychain fills.
type memKeys struct {
	mu sync.Mutex
	m  map[string]string
}

func (k *memKeys) Load(n string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[n]
	if !ok {
		return "", ErrNoKey
	}
	return v, nil
}

func (k *memKeys) Save(n, v string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.m == nil {
		k.m = map[string]string{}
	}
	k.m[n] = v
	return nil
}

func (k *memKeys) Clear(n string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, n)
	return nil
}

func (k *memKeys) Names() ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for n := range k.m {
		out = append(out, n)
	}
	return out, nil
}

func TestKeysListSetClear(t *testing.T) {
	accts, err := Open(filepath.Join(t.TempDir(), "Hearth"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var seen []Account
	accts.OnChange = func(a Account) { seen = append(seen, a) }

	list := accts.List()
	if len(list) != len(Providers) {
		t.Fatalf("List has %d accounts, want %d", len(list), len(Providers))
	}
	for _, a := range list {
		if a.State != SignedOut || a.Label != "" {
			t.Errorf("fresh account %+v, want signed out", a)
		}
	}

	if err := accts.SetKey(OpenRouter, "  "); err == nil {
		t.Error("SetKey accepted an empty key")
	}
	if err := accts.SetKey(OpenRouter, " sk-or-v1-abcdef1234567890 \n"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	got := accts.List()[3]
	if got.Provider != OpenRouter || got.State != SignedIn || got.Label != "7890" {
		t.Errorf("after SetKey = %+v, want signed in ending 7890", got)
	}
	if len(seen) != 1 || seen[0] != got {
		t.Errorf("OnChange saw %+v", seen)
	}

	// The key is on disk, owner only, under its name, trimmed.
	path := filepath.Join(accts.Keys.(FileKeys).Dir, "openrouter")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sk-or-v1-abcdef1234567890" {
		t.Errorf("stored key = %q, %v", data, err)
	}

	if err := accts.ClearKey(OpenRouter); err != nil {
		t.Fatal(err)
	}
	if got := accts.List()[3]; got.State != SignedOut || got.Label != "" {
		t.Errorf("after ClearKey = %+v", got)
	}
	if len(seen) != 2 || seen[1].State != SignedOut {
		t.Errorf("OnChange saw %+v", seen)
	}

	for _, p := range []Provider{ChatGPT, "nope"} {
		if err := accts.SetKey(p, "sk-ant-abcdef1234567890"); err == nil {
			t.Errorf("SetKey(%q) succeeded", p)
		}
	}
	for _, p := range []Provider{Anthropic, Gemini} {
		if err := accts.SetKey(p, "sk-ant-abcdef1234567890"); err != nil {
			t.Errorf("SetKey(%q) = %v, want the linked backend to accept it", p, err)
		}
	}
	if err := accts.ClearKey(ChatGPT); err == nil {
		t.Error("ClearKey(ChatGPT) succeeded")
	}
}

func TestResolveTiersFromAKey(t *testing.T) {
	accts := &Accounts{Keys: &memKeys{}}
	accts.chat = mustChat(t)
	if _, err := accts.Resolve(Fast); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("Resolve with nothing = %v, want no account", err)
	}
	if err := accts.SetKey(OpenRouter, "sk-or-v1-abcdef1234567890"); err != nil {
		t.Fatal(err)
	}
	d := llm.Defaults("openrouter")
	for tier, want := range map[Tier]string{Fast: d.CheapModel, Balanced: d.Model, Best: d.Model} {
		m, err := accts.Resolve(tier)
		if err != nil || m.Account != OpenRouter || m.Name != want || m.Name == "" {
			t.Errorf("Resolve(%d) = %+v, %v; want %q", tier, m, err, want)
		}
	}
	if _, err := accts.Resolve(Tier(0)); err == nil {
		t.Error("Resolve accepted an unknown tier")
	}
}

func mustChat(t *testing.T) *chatgpt.Account {
	t.Helper()
	a, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a.chat
}

// recordingProvider keeps the options it was streamed with.
type recordingProvider struct {
	fakeProvider
	opts llm.CompletionOpts
}

func (p *recordingProvider) StreamCompletion(ctx context.Context, msgs []llm.Message, o llm.CompletionOpts) (<-chan llm.StreamEvent, error) {
	p.opts = o
	return p.fakeProvider.StreamCompletion(ctx, msgs, o)
}

func TestStreamFunc(t *testing.T) {
	rp := &recordingProvider{fakeProvider: fakeProvider{tokens: []string{"hi"}}}
	stream := StreamFunc(Model{Provider: rp, Name: "m-1"})
	tools := []llm.Tool{{Name: "files.read"}}
	ch, err := stream(context.Background(), []llm.Message{{Role: llm.RoleUser, Content: "x"}}, tools, llm.ToolChoiceAuto)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	for ev := range ch {
		out.WriteString(ev.Token)
	}
	if out.String() != "hi" || rp.opts.Model != "m-1" || len(rp.opts.Tools) != 1 || rp.opts.ToolChoice != llm.ToolChoiceAuto {
		t.Errorf("stream = %q with %+v", out.String(), rp.opts)
	}
}

func TestScrubCoversEveryKeyInTheStore(t *testing.T) {
	keys := &memKeys{}
	accts := &Accounts{Keys: keys, chat: mustChat(t)}
	scrub := accts.Scrub()
	const (
		provKey  = "provider-key-0123456789abcdef"
		integKey = "BSAintegration0123456789xyz"
	)
	msg := llm.Message{Role: llm.RoleTool, Content: "found " + provKey + " and " + integKey + " here"}
	if got := scrub(msg); got.Content != msg.Content {
		t.Fatalf("scrubbed with an empty store: %q", got.Content)
	}

	// One key from this package, one stored by another owner of the store,
	// both saved after Scrub was taken.
	if err := keys.Save("openrouter", provKey); err != nil {
		t.Fatal(err)
	}
	if err := keys.Save("brave-search", integKey); err != nil {
		t.Fatal(err)
	}
	got := scrub(msg)
	if strings.Contains(got.Content, provKey) || strings.Contains(got.Content, integKey) {
		t.Errorf("a key reached the message: %q", got.Content)
	}
	if !strings.Contains(got.Content, "found ") || !strings.Contains(got.Content, " here") {
		t.Errorf("scrub removed more than the keys: %q", got.Content)
	}
}
