package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// refusal is words for the person, capitalised and ended like a sentence,
// for a service that does not check out when it is saved. None holds the
// key or what the service sent.
type refusal string

func (r refusal) Error() string { return string(r) }

const (
	errNotCompatible = refusal("That address answered, but not as an OpenAI-compatible service. Check the address; many gateways need `/v1` at the end.")
	errKeyRefused    = refusal("The key was refused.")
	errUnreachable   = refusal("Could not reach that address.")
)

const (
	// checkWall bounds the models call a save waits on.
	checkWall = 15 * time.Second
	// listBytes bounds how much of a model list is read.
	listBytes = 4 << 20
)

// SaveProfile asks the service's address for its models with the key, then
// stores it as SetProfile does. Anything but a model list refuses the save
// in plain words and stores nothing. What is wrong with the fields
// themselves, and a missing key, are SetProfile's to say.
// See docs/model.md#providers-and-models
func (a *Accounts) SaveProfile(ctx context.Context, p Profile, key string) (Account, error) {
	base, local, err := checkBaseURL(p.BaseURL)
	if err != nil {
		return a.SetProfile(p, key)
	}
	key = strings.TrimSpace(key)
	if p.ID != "" {
		if _, err := a.loadProfile(p.ID); err != nil {
			return a.SetProfile(p, key)
		}
		if key == "" {
			key, _ = a.Keys.Load(profileKeyName(p.ID))
		}
	}
	if key == "" && !local {
		return a.SetProfile(p, key)
	}
	if _, err := a.listModels(ctx, base, key); err != nil {
		return Account{}, err
	}
	return a.SetProfile(p, key)
}

// listModels asks base for its models with key and returns their ids, or
// one of the refusals above.
func (a *Accounts) listModels(ctx context.Context, base, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, checkWall)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", nil)
	if err != nil {
		return nil, errNotCompatible
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient(a.client).Do(req)
	if err != nil {
		return nil, errUnreachable
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, errKeyRefused
	default:
		return nil, errNotCompatible
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, listBytes))
	if err != nil {
		return nil, errUnreachable
	}
	var list struct {
		Data *[]struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &list) != nil || list.Data == nil {
		return nil, errNotCompatible
	}
	ids := make([]string, 0, len(*list.Data))
	for _, m := range *list.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}
