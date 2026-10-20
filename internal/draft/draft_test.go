package draft_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/org"
)

// model is a scripted stream that answers with text and remembers what it
// was asked.
type model struct {
	reply string
	err   error

	msgs  []llm.Message
	tools []llm.Tool
}

func (m *model) stream(_ context.Context, msgs []llm.Message, tools []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
	m.msgs, m.tools = msgs, tools
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Token: m.reply}
	ch <- llm.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

const strict = "```json\n" + `{"name": " Wren ", "role": "Research Analyst", "job_description": "You find and sum up sources. You check each one.", "working_style": "Direct and brief; asks before guessing", "tier": "Best"}` + "\n```"

func TestDraft_ReadsTheStrictShape(t *testing.T) {
	m := &model{reply: "Here you go:\n" + strict + "\nHope that helps."}
	templates := []org.RoleTemplate{{ID: "writer", Role: "Writer", JobDescription: "Write {{clearly}}."}}
	got, err := draft.Draft(t.Context(), draft.Input{
		Description: "someone to read up on competitors", TemplateID: "writer",
		Tier: org.TierFast, Templates: templates, Stream: m.stream,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := draft.Result{
		Name: "Wren", Role: "Research Analyst", JobDescription: "You find and sum up sources. You check each one.",
		WorkingStyle: "Direct and brief; asks before guessing", Tier: org.TierBest,
	}
	if got != want {
		t.Errorf("Draft = %+v, want %+v", got, want)
	}
	if len(m.tools) != 0 {
		t.Errorf("the draft offered %d tools, want none", len(m.tools))
	}
	var system, prompt string
	for _, msg := range m.msgs {
		switch msg.Role {
		case "system":
			system = msg.Content
		case "user":
			prompt = msg.Content
		}
	}
	if prompt != "someone to read up on competitors" {
		t.Errorf("the model was sent %q, want the description as typed", prompt)
	}
	if !strings.Contains(system, "Write {{clearly}}.") || !strings.Contains(system, "working_style") {
		t.Errorf("the system text lacks the template or the shape:\n%s", system)
	}
}

func TestDraft_FillsWhatTheReplyLeavesOut(t *testing.T) {
	m := &model{reply: `{"job_description": "You help.", "tier": "turbo"}`}
	got, err := draft.Draft(t.Context(), draft.Input{Description: "help me", Tier: org.TierBalanced, Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	if got.JobDescription != "You help." || got.Tier != org.TierBalanced || got.Name != "" || got.WorkingStyle != "" {
		t.Errorf("Draft = %+v", got)
	}
}

func TestDraft_MalformedReplies(t *testing.T) {
	for name, reply := range map[string]string{
		"prose":                 "I would hire someone friendly.",
		"empty":                 "",
		"broken object":         `{"name": "Wren", "job_description": `,
		"no job description":    `{"name": "Wren", "role": "Writer"}`,
		"blank job description": `{"job_description": "  "}`,
		"an array":              `["a"]`,
		"wrong type":            `{"job_description": 7}`,
		"unknown field":         `{"job_description": "You help.", "extra": 1}`,
		"two objects":           `{"job_description": "You help."} {"job_description": "Again."}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := &model{reply: reply}
			_, err := draft.Draft(t.Context(), draft.Input{Description: "x", Tier: org.TierFast, Stream: m.stream})
			if !errors.Is(err, draft.ErrMalformed) {
				t.Errorf("err = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestDraft_ModelUnavailable(t *testing.T) {
	t.Run("no model", func(t *testing.T) {
		_, err := draft.Draft(t.Context(), draft.Input{Description: "x"})
		if !errors.Is(err, draft.ErrUnavailable) || !strings.Contains(err.Error(), "Settings") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("provider failure", func(t *testing.T) {
		m := &model{err: &failure.Failure{Class: failure.ClassAuth, Provider: "anthropic"}}
		_, err := draft.Draft(t.Context(), draft.Input{Description: "x", Stream: m.stream})
		if !errors.Is(err, draft.ErrUnavailable) || !strings.Contains(err.Error(), "the key was refused by Anthropic") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestDraft_ScrubsWhatIsSent(t *testing.T) {
	m := &model{reply: strict}
	scrub := func(msg agent.Message) agent.Message {
		msg.Text = strings.ReplaceAll(msg.Text, "sk-secret", "[redacted]")
		return msg
	}
	if _, err := draft.Draft(t.Context(), draft.Input{Description: "use sk-secret", Stream: m.stream, Scrub: scrub}); err != nil {
		t.Fatal(err)
	}
	for _, msg := range m.msgs {
		if strings.Contains(msg.Content, "sk-secret") {
			t.Errorf("the model was sent the secret: %q", msg.Content)
		}
	}
}
