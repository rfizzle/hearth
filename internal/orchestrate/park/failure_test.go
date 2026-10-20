package park

import (
	"errors"
	"fmt"
	"testing"

	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

func TestFailure(t *testing.T) {
	run := org.Run{ID: "r", BotID: "b", ConversationID: "c"}
	cases := []struct {
		name  string
		err   error
		title string // empty when the run leaves no item
	}{
		{"a provider out of reach", &failure.Failure{Class: failure.ClassNetwork, Provider: "openai"}, "Dana could not reach OpenAI"},
		{"a refused key", fmt.Errorf("turn: %w", &failure.Failure{Class: failure.ClassAuth, Provider: "openai"}),
			"Dana: the key was refused by OpenAI; sign in again or replace it"},
		{"the step cap", agent.ErrRoundCap, ""},
		{"the time cap", agent.ErrWallCap, ""},
		{"a stop", agent.ErrCancelled, ""},
		{"hearth's own failure", errors.New("something went wrong inside hearth: boom"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it, ok := Failure(run, "Dana", tc.err)
			if tc.title == "" {
				if ok {
					t.Fatalf("Failure = %+v, want no item", it)
				}
				return
			}
			if !ok || it.Kind != inbox.Failure || it.Title != tc.title || it.RunID != "r" || it.BotID != "b" ||
				it.ConversationID != "" || it.Effect != agent.Explain(tc.err) {
				t.Fatalf("Failure = %+v, %v, want the item titled %q", it, ok, tc.title)
			}
		})
	}
}
