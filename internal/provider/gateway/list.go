package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

var (
	errNotCompatible = refusal{"That address answered, but not as an OpenAI-compatible service. Check the address; many gateways need `/v1` at the end.", ErrList}
	errKeyRefused    = refusal{"The key was refused.", ErrList}
	errUnreachable   = refusal{"Could not reach that address.", ErrList}
)

const (
	// checkWall bounds the models call a save waits on.
	checkWall = 15 * time.Second
	// listBytes bounds how much of a model list is read.
	listBytes = 4 << 20
)

// ListModels asks base for its models with key, if it takes one, through c
// (nil is a plain client), and returns their ids. Any other answer is a
// refusal in plain words that wraps ErrList and never holds the key or what
// the service sent. base is an address CheckBaseURL accepted.
func ListModels(ctx context.Context, c *http.Client, base, key string) ([]string, error) {
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
	resp, err := httpClient(c).Do(req)
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
