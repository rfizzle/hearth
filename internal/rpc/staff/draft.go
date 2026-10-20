package staff

import (
	"context"

	"github.com/rfizzle/hearth/internal/org"
)

// DraftRequest is what the owner asked a bot to be drafted from.
type DraftRequest struct {
	Description string
	TemplateID  string // the template they started from; empty or unknown for a blank start
	Tier        org.Tier
}

// Draft is the proposal for a bot. Nothing in it is stored.
type Draft struct {
	Name           string
	Role           string
	JobDescription string
	WorkingStyle   string
	Tier           org.Tier
}

// Drafter makes a proposal from the owner's words, at the Fast tier. Its
// owner sets it, so the package that talks to a model is the one that is
// handed the model: a draft that could not be made because no model was
// reachable wraps wire.ErrUnavailable, through wire.Unavailable.
// See docs/architecture.md#1-layering
type Drafter func(ctx context.Context, r DraftRequest) (Draft, error)
