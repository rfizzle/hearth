package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/rfizzle/agentcore/secret"
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

// ErrNoAccount is what Resolve returns when nothing is signed in.
var ErrNoAccount = errors.New("no account is signed in")

// Model is a provider ready to stream and the model to ask it for.
type Model struct {
	Account  Provider
	Provider llm.Provider
	Name     string
}

// Resolve returns the model for a tier from the first signed-in provider's
// defaults: the cheap model for Fast, the main one for Balanced and Best.
// A mapping the person edits arrives with the Tiers screen.
func (a *Accounts) Resolve(tier Tier) (Model, error) {
	if tier < Fast || tier > Best {
		return Model{}, fmt.Errorf("unknown quality tier %d", tier)
	}
	var lastErr error
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
	if errors.Is(err, ErrNoKey) {
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
