package toolset

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/hearth/internal/orchestrate/bus"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/tools"
)

// shelf keeps projects in memory.
type shelf struct{ all []project.Project }

func (s *shelf) CreateProject(_ context.Context, p project.Project, _ org.Conversation) (project.Project, error) {
	p.ID = "p" + string(rune('0'+len(s.all)))
	s.all = append(s.all, p)
	return p, nil
}

func (s *shelf) Projects(context.Context, bool) ([]project.Project, error) { return s.all, nil }

func proposal(name string) tools.Call {
	return tools.Call{Args: json.RawMessage(`{"name":"` + name + `","goal":"g","context":"c","done_looks_like":"l","done_when":"w"}`)}
}

func TestPropose(t *testing.T) {
	db, b := &shelf{}, bus.New()
	events, cancel := b.Subscribe(0)
	defer cancel()
	status := func(context.Context, string) (project.Status, error) { return project.StatusOf(project.Counts{}), nil }
	ctx := context.Background()
	def := Propose(db, b, status, "bot", "conv")
	if res, err := def.Execute(ctx, proposal("Launch")); err != nil || res.Text == "" {
		t.Fatalf("propose = %+v, %v", res, err)
	}
	if len(db.all) != 1 {
		t.Fatalf("stored %d projects, want 1", len(db.all))
	}
	got := db.all[0]
	if got.State != project.Proposed || got.Name != "Launch" || got.ProposedByBotID != "bot" || got.ProposedInConversationID != "conv" || got.FolderPath != "" {
		t.Errorf("stored = %+v, want a proposed project of bot in conv with no folder", got)
	}
	if e := <-events; e.Body != (bus.ProjectChanged{Project: got, Status: project.StatusOf(project.Counts{})}) {
		t.Errorf("event = %+v, want the project published", e.Body)
	}

	if _, err := def.Execute(ctx, proposal("Launch")); !errors.Is(err, ErrAlreadyProposed) {
		t.Errorf("a second proposal = %v, want ErrAlreadyProposed", err)
	}
	if _, err := def.Execute(ctx, proposal(" Launch ")); !errors.Is(err, ErrAlreadyProposed) {
		t.Errorf("a repeat padded with spaces = %v, want ErrAlreadyProposed", err)
	}
	if len(db.all) != 1 {
		t.Errorf("stored %d projects after the refusals, want 1", len(db.all))
	}
	if _, err := def.Execute(ctx, proposal("Launch 2")); err != nil {
		t.Errorf("another name: %v", err)
	}
	if _, err := Propose(db, b, status, "bot", "other").Execute(ctx, proposal("Launch")); err != nil {
		t.Errorf("another conversation: %v", err)
	}
	// Once the first is decided the name is free again.
	db.all[0].State = project.Active
	if _, err := def.Execute(ctx, proposal("Launch")); err != nil {
		t.Errorf("after the first was confirmed: %v", err)
	}
}

func TestPropose_RefusesBeforeStoring(t *testing.T) {
	db := &shelf{}
	def := Propose(db, bus.New(), nil, "bot", "conv")
	if _, err := def.Execute(context.Background(), proposal(strings.Repeat("x", 300))); err == nil || len(db.all) != 0 {
		t.Errorf("a 300-character name: %v, stored %d", err, len(db.all))
	}
}
