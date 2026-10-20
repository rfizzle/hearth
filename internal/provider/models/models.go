package models

import (
	"errors"
	"strings"

	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/llm"
)

// Choice is what identifies a bot's model: the quality tier it asks for and
// the model it pins under Advanced, in the form Encode makes. Override may
// be empty.
type Choice struct {
	Tier     provider.Tier
	Override string
}

// Row is one tier's line in the owner's mapping.
type Row struct {
	Tier provider.Tier
	Pick provider.Pick
}

var errBadOverride = errors.New("the model pinned for this bot is not one hearth can read")

// Resolve returns the model for a bot: its override if it has one, else the
// mapping's row for its tier, else the defaults. A mapping row that cannot
// be built falls back to the defaults, so a signed-out provider does not
// stop the bot; an override that cannot is an error that says why.
func Resolve(a *provider.Accounts, c Choice, mapping []Row) (provider.Model, error) {
	if c.Override != "" {
		pick, ok := Parse(c.Override)
		if !ok {
			return provider.Model{}, errBadOverride
		}
		return a.ResolveModel(c.Tier, pick)
	}
	for _, r := range mapping {
		if r.Tier == c.Tier {
			if m, err := a.ResolveModel(c.Tier, r.Pick); err == nil {
				return m, nil
			}
		}
	}
	return a.Resolve(c.Tier)
}

// Encode writes a pick as the one string a bot stores: provider, service id
// and model, joined by colons. The model comes last, so its own colons
// (llama3:8b) survive.
func Encode(p provider.Pick) string {
	return string(p.Provider) + ":" + p.Profile + ":" + p.Model
}

// Parse reads what Encode wrote; ok is false for anything else.
func Parse(s string) (p provider.Pick, ok bool) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[2] == "" {
		return provider.Pick{}, false
	}
	return provider.Pick{Provider: provider.Provider(parts[0]), Profile: parts[1], Model: parts[2]}, true
}

// Option is one model the owner can pick, with the words to show for it.
type Option struct {
	Pick  provider.Pick
	Label string
}

// serviceModels is an added service's default model, then the models a
// detected runtime at its address lists.
func serviceModels(acct provider.Account, found []Service) []string {
	out := []string{acct.DefaultModel}
	for _, f := range found {
		if f.Detected && f.BaseURL == acct.BaseURL {
			out = append(out, f.Models...)
		}
	}
	return out
}

// Options lists every model the signed-in providers and the added services
// offer: a provider's main and cheap model from llm's defaults, an added
// service's default model, and any model that service lists in found (the
// detected runtimes, matched by address). Each model appears once.
func Options(a *provider.Accounts, found []Service) []Option {
	names := map[string]string{}
	for _, k := range provider.KnownServices() {
		names[k.ID] = k.Name
	}
	var out []Option
	seen := map[provider.Pick]bool{}
	add := func(p provider.Pick, label string) {
		if p.Model != "" && !seen[p] {
			seen[p] = true
			out = append(out, Option{Pick: p, Label: label})
		}
	}
	for _, acct := range a.List() {
		if acct.State != provider.SignedIn {
			continue
		}
		if acct.Provider == provider.Compatible {
			for _, m := range serviceModels(acct, found) {
				add(provider.Pick{Provider: provider.Compatible, Profile: acct.Profile, Model: m}, acct.DisplayName)
			}
			continue
		}
		d := llm.Defaults(string(acct.Provider))
		add(provider.Pick{Provider: acct.Provider, Model: d.Model}, names[string(acct.Provider)])
		add(provider.Pick{Provider: acct.Provider, Model: d.CheapModel}, names[string(acct.Provider)])
	}
	return out
}
