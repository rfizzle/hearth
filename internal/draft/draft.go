package draft

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/framing"
	"github.com/rfizzle/hearth/internal/org"
)

var (
	// ErrUnavailable is a draft the model could not make: no account is
	// signed in, or the provider failed. The message says what to do.
	ErrUnavailable = errors.New("the draft could not be made")
	// ErrMalformed is a reply that was not the object that was asked for.
	ErrMalformed = errors.New("the draft came back in a form that could not be read; try again")
)

// wall ends a draft that is taking too long; the person is waiting on it.
const wall = 60 * time.Second

// Input is one draft request.
type Input struct {
	Description string // what the owner typed
	TemplateID  string // the template they started from; empty or unknown for a blank start
	Tier        org.Tier
	Templates   []org.RoleTemplate
	Stream      agent.Stream
	Scrub       agent.Scrub
}

// Result is the proposal. Nothing in it is stored.
type Result struct {
	Name           string
	Role           string
	JobDescription string
	WorkingStyle   string
	Tier           org.Tier
}

// reply is the shape the model is asked for.
type reply struct {
	Name           string `json:"name"`
	Role           string `json:"role"`
	JobDescription string `json:"job_description"`
	WorkingStyle   string `json:"working_style"`
	Tier           string `json:"tier"`
}

var tiers = map[string]org.Tier{"fast": org.TierFast, "balanced": org.TierBalanced, "best": org.TierBest}

// Draft asks the model once and parses its answer. A model that cannot be
// reached is ErrUnavailable and an answer that cannot be read ErrMalformed,
// each wrapping the plain words.
func Draft(ctx context.Context, in Input) (Result, error) {
	fin := framing.DraftInput{}
	for _, t := range in.Templates {
		if t.ID == in.TemplateID {
			fin = framing.DraftInput{Role: t.Role, Starting: t.JobDescription}
		}
	}
	out, err := ask(ctx, framing.RenderDraft(fin), in.Description, in.Stream, in.Scrub)
	if err != nil {
		return Result{}, err
	}
	return parse(out, in.Tier)
}

// ask runs the one model call a draft makes: the framing as the system text
// and the owner's words as the prompt, so they stay data; no tools, one
// round, a wall. The words come back or an ErrUnavailable.
func ask(ctx context.Context, system, prompt string, stream agent.Stream, scrub agent.Scrub) (string, error) {
	if stream == nil {
		return "", fmt.Errorf("%w: no model is set up; sign in to one in Settings", ErrUnavailable)
	}
	out, err := agent.Run(ctx, agent.Input{
		System:    system,
		Prompt:    prompt,
		Stream:    stream,
		Scrub:     scrub,
		MaxRounds: 1,
		MaxWall:   wall,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrUnavailable, agent.Explain(err))
	}
	return out.Text, nil
}

// parse reads the model's text strictly: one object, no field the reply
// struct lacks, every string bounded (see bound.go). asked is the tier to keep
// when the reply names none that is known.
func parse(raw string, asked org.Tier) (Result, error) {
	r, err := read[reply](raw)
	if err != nil {
		return Result{}, err
	}
	res := Result{
		Name:           line(r.Name, maxName),
		Role:           line(r.Role, maxRole),
		JobDescription: multi(r.JobDescription, maxPart),
		WorkingStyle:   line(r.WorkingStyle, maxStyle),
		Tier:           asked,
	}
	if res.JobDescription == "" {
		return Result{}, ErrMalformed
	}
	if t, ok := tiers[strings.ToLower(strings.TrimSpace(r.Tier))]; ok {
		res.Tier = t
	}
	return res, nil
}
