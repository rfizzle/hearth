package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/chatgpt"
	"github.com/rfizzle/llm/chatgpt/credstore"
)

// Provider names a model provider the person can sign in to. The value is
// llm's name for it and the name its key is stored under.
type Provider string

// The providers, in the order Resolve prefers them.
// See docs/model.md#providers-and-models
const (
	ChatGPT    Provider = "chatgpt"
	Anthropic  Provider = "anthropic"
	Gemini     Provider = "gemini"
	OpenRouter Provider = "openrouter"
)

// Providers is every provider that has an account, in preference order.
var Providers = []Provider{ChatGPT, Anthropic, Gemini, OpenRouter}

// State is where an account stands.
type State int

// The states an account reports.
const (
	SignedOut State = iota + 1
	SigningIn
	SignedIn
)

// Account is what the core says about one provider. It never holds the
// credential.
type Account struct {
	Provider Provider
	State    State
	// Label is the signed-in email, or the last four characters of a key.
	Label string
	// Failure is why the last sign-in failed, in plain words, until the next
	// attempt starts.
	Failure string
	// Profile, DisplayName, BaseURL and DefaultModel describe a service the
	// person added (Provider is Compatible); they are empty for the
	// built-ins.
	Profile      string
	DisplayName  string
	BaseURL      string
	DefaultModel string
}

// Accounts is the person's accounts over one data directory. The orchestrator
// and the contract handlers share one.
type Accounts struct {
	// Keys holds the API keys, and whatever the integrations store beside
	// them. Open sets a store that is the files until the first Sync and
	// memory after.
	Keys keystore.Store
	// OnChange is told each time an account's state changes. Set it before
	// the first call; it runs on whichever goroutine made the change and
	// must not block.
	OnChange func(Account)
	// OnCredentialChange is told the name of each credential the memory
	// store stored, replaced or removed after the app's Sync, and whether it
	// was removed; never a value. Same rules as OnChange.
	OnCredentialChange func(name string, deleted bool)

	chat *chatgpt.Account
	// client is the network the added services are reached through, as it
	// was handed to Open.
	client *http.Client
	// login carries the listener and wait seams for the browser sign-in.
	login chatgpt.LoginOptions

	mu      sync.Mutex
	attempt *attempt
	// bg counts the background sign-ins so a test can wait for them before
	// its temporary directory is removed.
	bg      sync.WaitGroup
	failure map[Provider]string
	// profileMu serialises adding, changing and removing a service, so an
	// id is minted once.
	profileMu sync.Mutex

	// The credential hand-shake (sync.go). keys and store are the routers
	// Keys and chat hold; mem is nil until the first Sync.
	syncMu    sync.Mutex
	keys      *keystore.Router
	store     *keystore.ChatRouter
	fileKeys  keystore.FileKeys
	fileChat  credstore.FileStore
	mem       *keystore.MemoryKeys
	filesGone bool
}

// attempt is one browser sign-in in flight.
type attempt struct {
	once  sync.Once
	ready chan struct{} // closed when page or err is set
	page  string
	err   error
}

// Open opens the accounts kept in dir: the ChatGPT sign-in in dir/chatgpt
// and the API keys, and the services added by address, in dir/keys. These
// files serve a core run by hand; the first Sync moves the accounts to
// memory (D29). client
// reaches the authorization server and the added services; nil is the real
// network.
func Open(dir string, client *http.Client) (*Accounts, error) {
	fileChat := credstore.FileStore{Dir: filepath.Join(dir, "chatgpt")}
	fileKeys := keystore.FileKeys{Dir: filepath.Join(dir, "keys")}
	store := keystore.NewChatRouter(fileChat)
	chat, err := chatgpt.Open(chatgpt.Options{Store: store, Client: client})
	if err != nil {
		return nil, err
	}
	keys := keystore.NewRouter(fileKeys)
	return &Accounts{
		Keys:     keys,
		chat:     chat,
		client:   client,
		failure:  map[Provider]string{},
		keys:     keys,
		store:    store,
		fileKeys: fileKeys,
		fileChat: fileChat,
	}, nil
}

// List reports every provider's account, then one per service the person
// added, the first added first.
func (a *Accounts) List() []Account {
	profiles := a.Profiles()
	out := make([]Account, 0, len(Providers)+len(profiles))
	for _, p := range Providers {
		out = append(out, a.account(p))
	}
	for _, p := range profiles {
		out = append(out, a.profileAccount(p))
	}
	return out
}

func (a *Accounts) account(p Provider) Account {
	a.mu.Lock()
	signing := p == ChatGPT && a.attempt != nil
	failure := a.failure[p]
	a.mu.Unlock()
	acct := Account{Provider: p, State: SignedOut, Failure: failure}
	if signing {
		acct.State = SigningIn
		return acct
	}
	if p == ChatGPT {
		cred, err := a.chat.Credential()
		switch {
		case err != nil:
			acct.Failure = Explain(err)
		case cred.SignedIn():
			acct.State, acct.Label = SignedIn, cred.Email
		}
		return acct
	}
	if key, err := a.Keys.Load(string(p)); err == nil {
		acct.State, acct.Label = SignedIn, lastFour(key)
	} else if !errors.Is(err, keystore.ErrNoKey) {
		acct.Failure = err.Error()
	}
	return acct
}

// lastFour is the tail of a key, or nothing when the key is too short for a
// tail to be safe to show.
func lastFour(key string) string {
	if len(key) < 12 {
		return ""
	}
	return key[len(key)-4:]
}

func (a *Accounts) publish(p Provider) {
	if a.OnChange != nil {
		a.OnChange(a.account(p))
	}
}

// SetKey stores the key for a provider that signs in with one.
func (a *Accounts) SetKey(p Provider, key string) error {
	if err := keyed(p); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("paste the key first, the field was empty")
	}
	if err := a.Keys.Save(string(p), key); err != nil {
		return err
	}
	a.mu.Lock()
	delete(a.failure, p)
	a.mu.Unlock()
	a.publish(p)
	return nil
}

// ClearKey removes the stored key for a provider.
func (a *Accounts) ClearKey(p Provider) error {
	if p != ChatGPT && !known(p) {
		return fmt.Errorf("%s is not a provider", p)
	}
	if p == ChatGPT {
		return errors.New("ChatGPT signs in with your account; sign out instead")
	}
	if err := a.Keys.Clear(string(p)); err != nil {
		return err
	}
	a.publish(p)
	return nil
}

// keyed reports why p cannot take a key, or nil.
func keyed(p Provider) error {
	switch {
	case p == ChatGPT:
		return errors.New("ChatGPT signs in with your account, not a key")
	case !known(p):
		return fmt.Errorf("%s is not a provider", p)
	case !llm.IsBuiltin(string(p)):
		return fmt.Errorf("keys for %s are not supported in this build yet", p)
	}
	return nil
}

func known(p Provider) bool {
	for _, k := range Providers {
		if k == p {
			return true
		}
	}
	return false
}

// Logout signs out of ChatGPT and reports whether the server confirmed it.
func (a *Accounts) Logout(ctx context.Context) (revoked bool, err error) {
	revoked, err = a.chat.Logout(ctx)
	if err == nil {
		a.mu.Lock()
		delete(a.failure, ChatGPT)
		a.mu.Unlock()
	}
	a.publish(ChatGPT)
	return revoked, err
}

// Login runs the browser sign-in and waits for it, telling the person who
// signed in on o.Say, their terminal. The helper's command line uses it.
func (a *Accounts) Login(ctx context.Context, o chatgpt.LoginOptions) error {
	at, ok := a.begin()
	if !ok {
		return errors.New("a ChatGPT sign-in is already waiting for the browser")
	}
	res, err := a.signIn(ctx, at, o)
	if err != nil {
		return err
	}
	if o.Say != nil {
		_, err = fmt.Fprintf(o.Say, "Signed in to ChatGPT as %s.\n", res.Email)
	}
	return err
}

// StartLogin begins the ChatGPT sign-in and returns the page the person
// signs in on as soon as it is known, so the app can open it. The sign-in
// finishes in the background and OnChange hears how it ended. A second call
// while one is waiting returns the same page.
func (a *Accounts) StartLogin(ctx context.Context) (authorizeURL string, err error) {
	at, started := a.begin()
	if started {
		o := a.login
		o.Browser = func(page string) error {
			at.once.Do(func() { at.page = page; close(at.ready) })
			return nil
		}
		o.StopWords = "Close the sign-in window to stop"
		// The call's context ends with the request, and the sign-in must
		// outlive it; the wait bound ends it instead.
		a.bg.Add(1)
		go func() {
			defer a.bg.Done()
			_, _ = a.signIn(context.WithoutCancel(ctx), at, o)
		}()
	}
	select {
	case <-at.ready:
		if at.err != nil {
			return "", at.err
		}
		return at.page, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// begin claims the one sign-in slot. It reports false when one is running.
func (a *Accounts) begin() (*attempt, bool) {
	a.mu.Lock()
	if a.attempt != nil {
		at := a.attempt
		a.mu.Unlock()
		return at, false
	}
	at := &attempt{ready: make(chan struct{})}
	a.attempt = at
	delete(a.failure, ChatGPT)
	a.mu.Unlock()
	a.publish(ChatGPT)
	return at, true
}

// signIn runs the sign-in for the attempt begin returned, then frees the
// slot and publishes how it ended.
func (a *Accounts) signIn(ctx context.Context, at *attempt, o chatgpt.LoginOptions) (chatgpt.Result, error) {
	res, err := a.chat.Login(ctx, o)
	a.mu.Lock()
	a.attempt = nil
	if err != nil {
		a.failure[ChatGPT] = Explain(err)
	}
	a.mu.Unlock()
	// A failure before the browser was asked releases a caller still waiting
	// for the page.
	at.once.Do(func() {
		at.err = errors.New("the sign-in finished without a page to open")
		if err != nil {
			at.err = errors.New(Explain(err))
		}
		close(at.ready)
	})
	a.publish(ChatGPT)
	return res, err
}
