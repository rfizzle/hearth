package draft

import (
	"context"
	"encoding/json"
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
	if in.Stream == nil {
		return Result{}, fmt.Errorf("%w: no model is set up; sign in to one in Settings", ErrUnavailable)
	}
	fin := framing.DraftInput{}
	for _, t := range in.Templates {
		if t.ID == in.TemplateID {
			fin = framing.DraftInput{Role: t.Role, Starting: t.JobDescription}
		}
	}
	out, err := agent.Run(ctx, agent.Input{
		System:    framing.RenderDraft(fin),
		Prompt:    in.Description,
		Stream:    in.Stream,
		Scrub:     in.Scrub,
		MaxRounds: 1,
		MaxWall:   wall,
	})
	if err != nil {
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, agent.Explain(err))
	}
	return parse(out.Text, in.Tier)
}

// parse reads the model's text defensively. asked is the tier to keep when
// the reply names none that is known.
func parse(text string, asked org.Tier) (Result, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return Result{}, ErrMalformed
	}
	var r reply
	if err := json.Unmarshal([]byte(text[start:end+1]), &r); err != nil {
		return Result{}, ErrMalformed
	}
	res := Result{
		Name:           strings.TrimSpace(r.Name),
		Role:           strings.TrimSpace(r.Role),
		JobDescription: strings.TrimSpace(r.JobDescription),
		WorkingStyle:   strings.TrimSpace(r.WorkingStyle),
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
