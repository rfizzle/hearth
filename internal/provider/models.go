package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/rfizzle/hearth/internal/keystore"
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

// ServiceModels lists the ids of the models a service offers, through the
// accounts' client, so a card can offer a picker before the service is
// saved. With profile set it asks the saved service at its stored address
// with its stored key, and base and key are ignored. Otherwise it asks base
// with key, if the service takes one, after the check that keeps a key off
// plain http; the key is used for this call and kept nowhere. An unknown
// profile is ErrNoProfile, a refused address wraps gateway.ErrAddress, and a
// list that could not be read wraps gateway.ErrList; each in plain words.
// It is also how a local runtime is detected; the caller bounds the wait
// with ctx.
// See docs/model.md#providers-and-models
func (a *Accounts) ServiceModels(ctx context.Context, profile, base, key string) ([]string, error) {
	if profile != "" {
		f, err := a.loadProfile(profile)
		if errors.Is(err, keystore.ErrNoKey) {
			return nil, ErrNoProfile
		}
		if err != nil {
			return nil, err
		}
		// A local service may have no key; the list is asked without one.
		key, _ = a.Keys.Load(profileKeyName(profile))
		return gateway.ListModels(ctx, a.client, f.BaseURL, key)
	}
	base, _, err := gateway.CheckBaseURL(base)
	if err != nil {
		return nil, err
	}
	return gateway.ListModels(ctx, a.client, base, strings.TrimSpace(key))
}
