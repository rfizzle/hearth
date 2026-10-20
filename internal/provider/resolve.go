package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/rfizzle/agentcore/secret"
	"github.com/rfizzle/hearth/internal/keystore"
	"github.com/rfizzle/llm"
)

// Tier is the quality a bot asks for; the org maps it to a model. The
// orchestrator swaps this for the org package's type when that lands.
type Tier int

// The tiers, from cheapest to best.
const (
	Fast Tier = iota + 1
	Balanced
	Best
)

// Pick is one model on one provider, the answer to "which model": the
// provider, the added service's id when it is Compatible, and the model.
// The zero Pick means the defaults.
type Pick struct {
	Provider Provider
	Profile  string
	Model    string
}

// ErrNoAccount is what Resolve returns when nothing is signed in.
var ErrNoAccount = errors.New("no account is signed in")

// Model is a provider ready to stream and the model to ask it for.
type Model struct {
	Account Provider
	// Profile is the added service's id when Account is Compatible.
	Profile  string
	Provider llm.Provider
	Name     string
}

// Resolve returns the model for a tier. A service the person added is
// preferred, the first one added, at its default model for every tier: it
// is the one they chose to bring. Otherwise it is the first signed-in
// provider's defaults: the cheap model for Fast, the main one for Balanced
// and Best. Resolve is the defaults; ResolveModel takes a model the person
// chose.
// See docs/model.md#providers-and-models
func (a *Accounts) Resolve(tier Tier) (Model, error) { return a.ResolveModel(tier, Pick{}) }

// ResolveModel returns pick's model when pick names one, and what Resolve
// returns for the tier when it is the zero Pick. A pick whose provider is
// not signed in, or whose service was removed, is an error in plain words.
func (a *Accounts) ResolveModel(tier Tier, pick Pick) (Model, error) {
	if tier < Fast || tier > Best {
		return Model{}, fmt.Errorf("unknown quality tier %d", tier)
	}
	if pick != (Pick{}) {
		return a.pinned(pick)
	}
	var lastErr error
	for _, p := range a.Profiles() {
		prov, err := a.buildProfile(p)
		if err != nil {
			lastErr = err
			continue
		}
		return Model{Account: Compatible, Profile: p.ID, Provider: prov, Name: p.DefaultModel}, nil
	}
	for _, p := range Providers {
		prov, err := a.build(p)
		if errors.Is(err, ErrNoAccount) {
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		d := llm.Defaults(string(p))
		name := d.Model
		if tier == Fast && d.CheapModel != "" {
			name = d.CheapModel
		}
		return Model{Account: p, Provider: prov, Name: name}, nil
	}
	if lastErr != nil {
		return Model{}, lastErr
	}
	return Model{}, ErrNoAccount
}

// pinned builds the provider for a chosen model.
func (a *Accounts) pinned(p Pick) (Model, error) {
	if p.Provider == Compatible {
		f, err := a.loadProfile(p.Profile)
		if err != nil {
			return Model{}, errors.New("the service that model is on was removed")
		}
		prov, err := a.buildProfile(Profile{ID: p.Profile, Name: f.Name, BaseURL: f.BaseURL, DefaultModel: p.Model})
		return Model{Account: Compatible, Profile: p.Profile, Provider: prov, Name: p.Model}, err
	}
	prov, err := a.build(p.Provider)
	if errors.Is(err, ErrNoAccount) {
		return Model{}, fmt.Errorf("%s is not signed in", p.Provider)
	}
	return Model{Account: p.Provider, Provider: prov, Name: p.Model}, err
}

// build makes the llm provider for a signed-in account, or ErrNoAccount.
func (a *Accounts) build(p Provider) (llm.Provider, error) {
	if p == ChatGPT {
		cred, err := a.chat.Credential()
		if err != nil || !cred.SignedIn() {
			return nil, ErrNoAccount
		}
		return llm.Resolve(string(p), llm.ResolveOpts{TokenSource: a.chat})
	}
	key, err := a.Keys.Load(string(p))
	if errors.Is(err, keystore.ErrNoKey) {
		return nil, ErrNoAccount
	}
	if err != nil {
		return nil, err
	}
	return llm.Resolve(string(p), llm.ResolveOpts{APIKey: key})
}

// StreamFunc returns the function that opens one completion on m, in llm's
// terms, for the orchestrator to hand to the agent loop.
func StreamFunc(m Model) func(ctx context.Context, msgs []llm.Message, tools []llm.Tool, choice string) (<-chan llm.StreamEvent, error) {
	return func(ctx context.Context, msgs []llm.Message, tools []llm.Tool, choice string) (<-chan llm.StreamEvent, error) {
		return m.Provider.StreamCompletion(ctx, msgs, llm.CompletionOpts{Model: m.Name, Tools: tools, ToolChoice: choice})
	}
}

// Scrub returns a function that redacts every secret the core holds from a
// message: each key in the store, whoever owns it, and the ChatGPT tokens,
// plus the shapes of well-known credentials. The vault is built on each call
// from the store, so a key saved a moment ago is already covered. A key the
// store cannot be read for is skipped; the shape patterns still run.
func (a *Accounts) Scrub() func(llm.Message) llm.Message {
	return func(m llm.Message) llm.Message { return a.vault().ScrubMessage(m) }
}

func (a *Accounts) vault() *secret.Vault {
	v := secret.New()
	names, _ := a.Keys.Names()
	for _, n := range names {
		if isProfileMeta(n) {
			// A service's name and address, not a secret.
			continue
		}
		key, err := a.Keys.Load(n)
		if err != nil || key == "" {
			continue
		}
		if v.Add(n, key) != nil {
			v.Hide("key", key)
		}
	}
	if cred, err := a.chat.Credential(); err == nil {
		for _, t := range []string{cred.AccessToken, cred.RefreshToken, cred.IDToken} {
			if t != "" {
				v.Hide("chatgpt", t)
			}
		}
	}
	return v
}
