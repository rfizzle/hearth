package staff

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// maxDescription is the longest paragraph a project is drafted from, in
// characters: a paragraph, not a document. The request is the owner's, but
// what is in it goes to a model, so the handler bounds it before anything
// else is done with it.
const maxDescription = 4000

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
	Concerns       []concern.Concern // the model's, as the drafter parsed them; the handler caps them behind the rules'
}

// Drafter makes a proposal from the owner's words, at the Fast tier. Its
// owner sets it, so the package that talks to a model is the one that is
// handed the model: a draft that could not be made because no model was
// reachable wraps wire.ErrUnavailable, through wire.Unavailable.
// See docs/architecture.md#1-layering
type Drafter func(ctx context.Context, r DraftRequest) (Draft, error)

// ProjectDraft is the proposal for a project. Nothing in it is stored.
// Concerns are the model's, as the drafter parsed them; the handler caps them
// behind the rules'.
type ProjectDraft struct {
	Name       string
	Brief      project.Brief
	TargetDate string // yyyy-mm-dd, or empty
	Concerns   []concern.Concern
}

// ProjectDrafter makes a proposal from the owner's paragraph, at the Fast
// tier. Like Drafter, a draft that could not be made because no model was
// reachable wraps wire.ErrUnavailable, through wire.Unavailable.
// See docs/architecture.md#1-layering
type ProjectDrafter func(ctx context.Context, description string) (ProjectDraft, error)

// askable is the refusal for a draft request that should not reach a model:
// nothing to draft from, a paragraph over limit characters (none when limit
// is 0), or no drafter set, which is unavailable in the words the owner reads.
func askable(description string, limit int, have bool) error {
	if err := wire.Need("description", description); err != nil {
		return err
	}
	if limit > 0 && utf8.RuneCountInString(description) > limit {
		return wire.Invalid("description is over %d characters; say it in a paragraph", limit)
	}
	if !have {
		return wire.Fail(wire.Unavailable(errors.New("no model is set up; sign in to one in Settings")))
	}
	return nil
}
