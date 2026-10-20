package provider

import (
	"context"
	"strings"

	"github.com/rfizzle/hearth/internal/provider/gateway"
)

// SaveProfile asks the service's address for its models with the key, then
// stores it as SetProfile does. Anything but a model list refuses the save
// in plain words and stores nothing. What is wrong with the fields
// themselves, and a missing key, are SetProfile's to say.
// See docs/model.md#providers-and-models
func (a *Accounts) SaveProfile(ctx context.Context, p Profile, key string) (Account, error) {
	base, local, err := gateway.CheckBaseURL(p.BaseURL)
	if err != nil {
		return a.SetProfile(p, key)
	}
	key = strings.TrimSpace(key)
	if p.ID != "" {
		old, err := a.loadProfile(p.ID)
		if err != nil || (key == "" && gateway.HostMoved(old.BaseURL, base)) {
			return a.SetProfile(p, key)
		}
		if key == "" {
			key, _ = a.Keys.Load(profileKeyName(p.ID))
		}
	}
	if key == "" && !local {
		return a.SetProfile(p, key)
	}
	if _, err := gateway.ListModels(ctx, a.client, base, key); err != nil {
		return Account{}, err
	}
	return a.SetProfile(p, key)
}

// ListModels asks the service at base, with key if it takes one, for the ids
// of its models, through the accounts' client. It is how a local runtime is
// detected; the caller bounds the wait with ctx.
func (a *Accounts) ListModels(ctx context.Context, base, key string) ([]string, error) {
	return gateway.ListModels(ctx, a.client, base, key)
}
