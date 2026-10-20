package draft_test

import (
	"reflect"
	"testing"

	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/org"
)

const concernsArray = `"concerns": [` +
	`{"field": "goal", "concern": "Too broad.", "suggestion": "Ship the first issue."}, ` +
	`{"field": "tier", "concern": "Not a field.", "suggestion": ""}]`

var wantConcerns = []concern.Concern{
	{Field: "goal", Concern: "Too broad.", Suggestion: "Ship the first issue."},
	{Field: "tier", Concern: "Not a field.", Suggestion: ""},
}

// The drafter reads the concerns as the model sent them; concern.Cap, which
// the handler runs, is what drops the unknown field and bounds the rest.
func TestDraftProject_ReadsTheModelsConcerns(t *testing.T) {
	m := &model{reply: `{"goal": "Ship it.", "done_when": "It is shipped.", ` + concernsArray + `}`}
	got, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "ship it", Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Concerns, wantConcerns) {
		t.Errorf("Concerns = %+v, want %+v", got.Concerns, wantConcerns)
	}
}

func TestDraft_ReadsTheModelsConcerns(t *testing.T) {
	m := &model{reply: `{"job_description": "You help.", ` + concernsArray + `}`}
	got, err := draft.Draft(t.Context(), draft.Input{Description: "help", Tier: org.TierFast, Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Concerns, wantConcerns) {
		t.Errorf("Concerns = %+v, want %+v", got.Concerns, wantConcerns)
	}
}

// Concerns are advice beside a usable draft, so a value that is not a list of
// concerns is none, not a malformed reply.
func TestDraft_ConcernsThatAreNotAListAreNone(t *testing.T) {
	for name, val := range map[string]string{
		"empty list":         `[]`,
		"null":               `null`,
		"a string":           `"none"`,
		"an object":          `{"field": "role"}`,
		"a list of strings":  `["too thin"]`,
		"a field of a type":  `[{"field": 7, "concern": "x"}]`,
		"a number":           `3`,
		"an unlisted extras": `[{"field": "role", "concern": "x", "extra": 1}]`,
	} {
		t.Run(name, func(t *testing.T) {
			m := &model{reply: `{"job_description": "You help.", "concerns": ` + val + `}`}
			got, err := draft.Draft(t.Context(), draft.Input{Description: "help", Tier: org.TierFast, Stream: m.stream})
			if err != nil {
				t.Fatalf("err = %v, want a usable draft", err)
			}
			if name == "an unlisted extras" {
				if len(got.Concerns) != 1 {
					t.Errorf("Concerns = %+v, want the one concern with its extra field ignored", got.Concerns)
				}
				return
			}
			if len(got.Concerns) != 0 {
				t.Errorf("Concerns = %+v, want none", got.Concerns)
			}
		})
	}
}

func TestDraft_ReadsAtMostAFewTimesTheCap(t *testing.T) {
	reply := `{"job_description": "You help.", "concerns": [`
	for range 40 {
		reply += `{"field": "role", "concern": "x"},`
	}
	reply = reply[:len(reply)-1] + `]}`
	m := &model{reply: reply}
	got, err := draft.Draft(t.Context(), draft.Input{Description: "help", Tier: org.TierFast, Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Concerns) != concern.Max*4 {
		t.Errorf("read %d concerns from 40, want %d", len(got.Concerns), concern.Max*4)
	}
}
