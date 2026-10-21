package draft_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/failure"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/draft"
	"github.com/rfizzle/hearth/internal/project"
)

const strictProject = "```json\n" + `{"name": " Launch the API tier ", "goal": "Send the launch email.", "context": "400 customers.", ` +
	`"done_looks_like": "A pricing page and a launch email.", "done_when": "The launch email is in 400 inboxes.", "target_date": "2026-11-20"}` + "\n```"

func TestDraftProject_ReadsTheStrictShape(t *testing.T) {
	m := &model{reply: "Here you go:\n" + strictProject + "\nHope that helps."}
	got, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "launch our API tier", Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	want := draft.ProjectResult{
		Name: "Launch the API tier",
		Brief: project.Brief{
			Goal: "Send the launch email.", Context: "400 customers.",
			DoneLooksLike: "A pricing page and a launch email.", DoneWhen: "The launch email is in 400 inboxes.",
		},
		TargetDate: "2026-11-20",
	}
	if got.Name != want.Name || got.Brief != want.Brief || got.TargetDate != want.TargetDate || len(got.Concerns) != 0 {
		t.Errorf("DraftProject = %+v, want %+v", got, want)
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
	if prompt != "launch our API tier" {
		t.Errorf("the model was sent %q, want the paragraph as typed", prompt)
	}
	if !strings.Contains(system, "done_looks_like") || strings.Contains(system, "launch our API tier") {
		t.Errorf("the system text lacks the shape or holds the paragraph:\n%s", system)
	}
}

func TestDraftProject_AReplyMissingTheDateHasNone(t *testing.T) {
	m := &model{reply: `{"goal": "Ship it.", "done_when": "It is shipped.", "target_date": ""}`}
	got, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "ship it", Stream: m.stream})
	if err != nil || got.TargetDate != "" || got.Name != "" || got.Brief.Context != "" {
		t.Errorf("DraftProject = %+v, %v, want empty optional fields", got, err)
	}
	m = &model{reply: `{"goal": "Ship it.", "done_when": "It is shipped."}`}
	if got, err = draft.DraftProject(t.Context(), draft.ProjectInput{Description: "ship it", Stream: m.stream}); err != nil || got.TargetDate != "" {
		t.Errorf("a reply with no target_date at all = %+v, %v", got, err)
	}
}

func TestDraftProject_ADateThatIsNotADayIsLeftForTheOwner(t *testing.T) {
	for _, d := range []string{"tomorrow", "2026-13-01", "2026-11-31", "11/20/2026", "2026-11-20T09:00:00Z"} {
		m := &model{reply: `{"goal": "g", "done_when": "d", "target_date": "` + d + `"}`}
		got, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x", Stream: m.stream})
		if err != nil || got.TargetDate != "" {
			t.Errorf("target_date %q = %q, %v, want it emptied", d, got.TargetDate, err)
		}
	}
}

func TestDraftProject_BoundsEveryStringByCuttingNotRefusing(t *testing.T) {
	long := strings.Repeat("é", 5000)
	m := &model{reply: `{"name": "` + strings.Repeat("n", 200) + `\n` + `", "goal": "` + long + `", "context": "` + long +
		`", "done_looks_like": "` + long + `", "done_when": "One\nline ` + long + `"}`}
	got, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x", Stream: m.stream})
	if err != nil {
		t.Fatalf("a long reply was refused: %v", err)
	}
	for what, c := range map[string]struct {
		s string
		n int
	}{
		"name": {got.Name, 80}, "goal": {got.Brief.Goal, 4000}, "context": {got.Brief.Context, 4000},
		"done_looks_like": {got.Brief.DoneLooksLike, 4000}, "done_when": {got.Brief.DoneWhen, 4000},
	} {
		if n := utf8.RuneCountInString(c.s); n != c.n {
			t.Errorf("%s has %d characters, want it cut to %d", what, n, c.n)
		}
	}
	if strings.ContainsAny(got.Name+got.Brief.DoneWhen, "\n\r") {
		t.Errorf("a one-line field kept a line break: %q, %q", got.Name, got.Brief.DoneWhen)
	}
	if err := project.Validate(got.Name, got.Brief, got.TargetDate); err != nil {
		t.Errorf("the cut draft is not a project the core would make: %v", err)
	}
}

func TestDraftProject_BotBoundsAreCutToo(t *testing.T) {
	long := strings.Repeat("x", 5000)
	m := &model{reply: `{"name": "` + long + `", "role": "` + long + `", "job_description": "` + long + `", "working_style": "A\nB ` + long + `"}`}
	got, err := draft.Draft(t.Context(), draft.Input{Description: "x", Stream: m.stream})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Name) != 80 || len(got.Role) != 80 || len(got.JobDescription) != 4000 || len(got.WorkingStyle) != 80 || strings.Contains(got.WorkingStyle, "\n") {
		t.Errorf("Draft lengths = %d %d %d %d, want 80 80 4000 80", len(got.Name), len(got.Role), len(got.JobDescription), len(got.WorkingStyle))
	}
}

func TestDraftProject_MalformedReplies(t *testing.T) {
	for name, reply := range map[string]string{
		"prose":            "I would start with a template.",
		"empty":            "",
		"broken object":    `{"goal": "Ship it", "done_when": `,
		"no goal":          `{"done_when": "It is shipped."}`,
		"blank goal":       `{"goal": "  ", "done_when": "It is shipped."}`,
		"no done_when":     `{"goal": "Ship it."}`,
		"blank done_when":  `{"goal": "Ship it.", "done_when": "\n"}`,
		"an array":         `["goal"]`,
		"wrong type":       `{"goal": 7, "done_when": "x"}`,
		"an unknown field": `{"goal": "g", "done_when": "d", "assign_to": "Wren"}`,
		"two objects":      `{"goal": "g", "done_when": "d"} and {"goal": "h", "done_when": "e"}`,
	} {
		t.Run(name, func(t *testing.T) {
			m := &model{reply: reply}
			_, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x", Stream: m.stream})
			if !errors.Is(err, draft.ErrMalformed) {
				t.Errorf("err = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestDraftProject_ModelUnavailable(t *testing.T) {
	t.Run("no model", func(t *testing.T) {
		_, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x"})
		if !errors.Is(err, draft.ErrUnavailable) || !strings.Contains(err.Error(), "Settings") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("provider failure", func(t *testing.T) {
		m := &model{err: &failure.Failure{Class: failure.ClassAuth, Provider: "anthropic"}}
		_, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x", Stream: m.stream})
		if !errors.Is(err, draft.ErrUnavailable) || !strings.Contains(err.Error(), "the key was refused by Anthropic") {
			t.Errorf("err = %v", err)
		}
	})
}

// A service that answers with something that is not a reply ends the stream
// with an error naming it; the draft says so.
func TestDraftProject_ServiceThatSendsNoReply(t *testing.T) {
	sent := errors.New(`Acme gateway answered, but not with a reply (it sent {"Output":{}}). Check the address and the model name`)
	stream := func(context.Context, []llm.Message, []llm.Tool, string) (<-chan llm.StreamEvent, error) {
		ch := make(chan llm.StreamEvent, 1)
		ch <- llm.StreamEvent{Done: true, Err: sent}
		close(ch)
		return ch, nil
	}
	_, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "x", Stream: stream})
	if !errors.Is(err, draft.ErrUnavailable) || errors.Is(err, draft.ErrMalformed) || !strings.Contains(err.Error(), "Acme gateway answered") {
		t.Errorf("err = %v, want the unavailable error carrying the service's words", err)
	}
}

// The draft ends after the wall even when the service never answers.
func TestDraftProject_StopsAtTheWall(t *testing.T) {
	stream := func(ctx context.Context, _ []llm.Message, _ []llm.Tool, _ string) (<-chan llm.StreamEvent, error) {
		ch := make(chan llm.StreamEvent)
		go func() {
			<-ctx.Done()
			ch <- llm.StreamEvent{Done: true, Err: ctx.Err()}
			close(ch)
		}()
		return ch, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := draft.DraftProject(ctx, draft.ProjectInput{Description: "x", Stream: stream})
	if !errors.Is(err, draft.ErrUnavailable) {
		t.Errorf("err = %v, want the unavailable error", err)
	}
}

func TestDraftProject_ScrubsWhatIsSent(t *testing.T) {
	m := &model{reply: strictProject}
	scrub := func(msg agent.Message) agent.Message {
		msg.Text = strings.ReplaceAll(msg.Text, "sk-secret", "[redacted]")
		return msg
	}
	if _, err := draft.DraftProject(t.Context(), draft.ProjectInput{Description: "use sk-secret", Stream: m.stream, Scrub: scrub}); err != nil {
		t.Fatal(err)
	}
	for _, msg := range m.msgs {
		if strings.Contains(msg.Content, "sk-secret") {
			t.Errorf("the model was sent the secret: %q", msg.Content)
		}
	}
}
