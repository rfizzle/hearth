package draft

import (
	"context"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/project"
)

// ProjectInput is one project draft request.
type ProjectInput struct {
	Description string // what the owner typed, at most 4,000 characters; the caller holds that bound
	Stream      agent.Stream
	Scrub       agent.Scrub
}

// ProjectResult is the proposal. Nothing in it is stored.
type ProjectResult struct {
	Name       string
	Brief      project.Brief
	TargetDate string // yyyy-mm-dd, or empty
	// Concerns are the model's, as it sent them and not yet capped or
	// checked; the caller passes them through concern.Cap with the rules'.
	Concerns []concern.Concern
}

// projectReply is the shape the model is asked for.
type projectReply struct {
	Name          string `json:"name"`
	Goal          string `json:"goal"`
	Context       string `json:"context"`
	DoneLooksLike string `json:"done_looks_like"`
	DoneWhen      string `json:"done_when"`
	TargetDate    string `json:"target_date"`
}

// DraftProject asks the model once and parses its answer. A model that cannot
// be reached is ErrUnavailable and an answer that cannot be read ErrMalformed,
// each wrapping the plain words. The paragraph is the prompt, never part of
// the system text, so it is data to the model.
func DraftProject(ctx context.Context, in ProjectInput) (ProjectResult, error) {
	out, err := ask(ctx, framing.RenderDraftProject(), in.Description, in.Stream, in.Scrub)
	if err != nil {
		return ProjectResult{}, err
	}
	return parseProject(out)
}

// parseProject reads the model's text strictly: one object, no field the
// reply struct lacks, every string bounded (see bound.go). The goal and the
// done-when line are required; any other field left out is empty, and the
// sheet asks for it.
func parseProject(raw string) (ProjectResult, error) {
	r, err := read[projectReply](raw)
	if err != nil {
		return ProjectResult{}, err
	}
	res := ProjectResult{
		Name: line(r.Name, maxName),
		Brief: project.Brief{
			Goal:          multi(r.Goal, maxPart),
			Context:       multi(r.Context, maxPart),
			DoneLooksLike: multi(r.DoneLooksLike, maxPart),
			DoneWhen:      line(r.DoneWhen, maxPart),
		},
		TargetDate: day(r.TargetDate),
	}
	if res.Brief.Goal == "" || res.Brief.DoneWhen == "" {
		return ProjectResult{}, ErrMalformed
	}
	return res, nil
}
