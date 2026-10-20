package models

import (
	"context"
	"time"

	"github.com/rfizzle/hearth/internal/provider"
)

// probeWait is how long Detect waits for a runtime on this Mac to answer.
const probeWait = 500 * time.Millisecond

// Service is a known service with what Detect found out about it. Detected
// and Models are set for a local runtime only.
type Service struct {
	provider.KnownService
	// Detected is whether the runtime answered a moment ago.
	Detected bool
	// Models are the ids it listed.
	Models []string
}

// Detect lists the known services and asks each local one, at the address in
// the table, for its models, through the accounts' client. The addresses
// are loopback, so nothing leaves this Mac. A runtime that does not answer
// in half a second, or not as a model list, is not detected.
func Detect(ctx context.Context, a *provider.Accounts) []Service {
	known := provider.KnownServices()
	out := make([]Service, len(known))
	for i, k := range known {
		out[i].KnownService = k
		if k.Kind != provider.Local {
			continue
		}
		func() {
			ctx, cancel := context.WithTimeout(ctx, probeWait)
			defer cancel()
			if ids, err := a.ListModels(ctx, k.BaseURL, ""); err == nil {
				out[i].Detected, out[i].Models = true, ids
			}
		}()
	}
	return out
}
