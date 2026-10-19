package provider

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"
)

// fakeAuth is auth.openai.com in memory: discovery, a key set, a token
// endpoint that issues an ID token for the person, and a revocation endpoint.
type fakeAuth struct {
	key     *ecdsa.PrivateKey
	mu      sync.Mutex
	nonce   string
	revoked int
}

func newFakeAuth(t *testing.T) *fakeAuth {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeAuth{key: key}
}

// client reaches the fake whatever host a request names, so nothing leaves
// the process.
func (f *fakeAuth) client() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		f.ServeHTTP(rec, r)
		return rec.Result(), nil
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func (f *fakeAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":              chatgpt.Issuer,
			"jwks_uri":            chatgpt.Issuer + "/jwks",
			"revocation_endpoint": chatgpt.Issuer + "/oauth/revoke",
		})
	case "/jwks":
		raw, _ := f.key.PublicKey.Bytes()
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "EC", "crv": "P-256", "kid": "k1", "alg": "ES256", "use": "sig",
			"x": base64.RawURLEncoding.EncodeToString(raw[1:33]),
			"y": base64.RawURLEncoding.EncodeToString(raw[33:]),
		}}})
	case "/api/accounts/oauth/token":
		_ = r.ParseForm()
		f.mu.Lock()
		nonce := f.nonce
		f.mu.Unlock()
		now := time.Now()
		id := f.sign(map[string]any{
			"iss": chatgpt.Issuer, "sub": "user-abc123", "aud": r.PostForm.Get("client_id"), "email": "person@example.com",
			"exp": now.Add(time.Hour).Unix(), "iat": now.Unix(), "nonce": nonce,
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at_example", "refresh_token": "rt_example", "id_token": id,
			"token_type": "Bearer", "expires_in": 3600, "scope": chatgpt.Scopes,
		})
	case "/oauth/revoke":
		f.mu.Lock()
		f.revoked++
		f.mu.Unlock()
	default:
		http.NotFound(w, r)
	}
}

// sign mints an ES256 JWT over claims.
func (f *fakeAuth) sign(claims map[string]any) string {
	part := func(v any) string {
		data, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(data)
	}
	signed := part(map[string]string{"alg": "ES256", "kid": "k1", "typ": "JWT"}) + "." + part(claims)
	digest := sha256.Sum256([]byte(signed))
	r, s, err := ecdsa.Sign(rand.Reader, f.key, digest[:])
	if err != nil {
		panic(err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signed + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// memListener is the loopback listener held in memory: the browser dials it
// with pipe, so the test opens no socket.
type memListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newMemListener() *memListener {
	return &memListener{conns: make(chan net.Conn), done: make(chan struct{})}
}

func (l *memListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *memListener) Close() error { l.once.Do(func() { close(l.done) }); return nil }

func (l *memListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 4242}
}

func (l *memListener) dial(context.Context, string, string) (net.Conn, error) {
	server, client := net.Pipe()
	select {
	case l.conns <- server:
		return client, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

// browse plays the person's browser: it reads the authorize URL and calls the
// loopback callback back as a person who approved.
func (f *fakeAuth) browse(l *memListener) func(string) error {
	return func(page string) error {
		u, err := url.Parse(page)
		if err != nil {
			return err
		}
		q := u.Query()
		f.mu.Lock()
		f.nonce = q.Get("nonce")
		f.mu.Unlock()
		back := url.Values{"state": {q.Get("state")}, "code": {"auth-code"}, "client_id": {"oaiapp_example"}}
		go func() {
			c := http.Client{Transport: &http.Transport{DialContext: l.dial}}
			resp, err := c.Get(q.Get("redirect_uri") + "?" + back.Encode())
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
		return nil
	}
}

type fakeProvider struct{ tokens []string }

func (fakeProvider) Name() string { return "fake" }

func (p fakeProvider) StreamCompletion(_ context.Context, msgs []llm.Message, _ llm.CompletionOpts) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, len(p.tokens)+1)
	for _, t := range p.tokens {
		ch <- llm.StreamEvent{Token: t}
	}
	ch <- llm.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

func TestSay(t *testing.T) {
	var out bytes.Buffer
	if err := Say(context.Background(), fakeProvider{tokens: []string{"hel", "lo"}}, "hi", &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello\n" {
		t.Errorf("Say wrote %q", out.String())
	}
}

func TestExplain(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"llm sentinel", llm.ErrChatGPTSignedOut, "Not signed in to ChatGPT. Run `hearthd login chatgpt`."},
		{"chatgpt sentinel", chatgpt.ErrNotSignedIn, "Not signed in to ChatGPT. Run `hearthd login chatgpt`."},
		{"wrapped", errors.Join(errors.New("resolve"), llm.ErrChatGPTSignedOut), "Not signed in to ChatGPT. Run `hearthd login chatgpt`."},
		{"other", errors.New("boom"), "boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Explain(tt.err); got != tt.want {
				t.Errorf("Explain = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenBrowserRefusesOtherPages(t *testing.T) {
	if err := OpenBrowser("https://example.com/"); err == nil {
		t.Fatal("OpenBrowser opened an address that is not the sign-in page")
	}
}
