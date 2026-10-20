package rpc

import (
	"context"
	"errors"

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
// reachable wraps ErrUnavailable, through Unavailable.
// See docs/architecture.md#1-layering
type Drafter func(ctx context.Context, r DraftRequest) (Draft, error)

// ErrUnavailable marks a draft that could not be made because no model was
// reachable: the owner has not signed in, or the provider failed.
var ErrUnavailable = errors.New("no model could be reached")

// Unavailable marks err as ErrUnavailable and keeps its words, so the owner
// reads what to do.
func Unavailable(err error) error { return unavailable{err} }

type unavailable struct{ error }

func (u unavailable) Unwrap() error        { return u.error }
func (u unavailable) Is(target error) bool { return target == ErrUnavailable }
