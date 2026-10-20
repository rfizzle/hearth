package orchestrate

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

const proposeArgs = `{"name":"Launch page","goal":"Ship the page","context":"Small team","done_looks_like":"A page is live","done_when":"The page loads."}`

func offers(t *testing.T, f *fixture, name string) bool {
	t.Helper()
	disp, err := f.svc.dispatcher(context.Background(), f.bot, f.bot.DMConversationID, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range disp.Tools() {
		if d.Name == name {
			return true
		}
	}
	return false
}

func TestProposeGrant(t *testing.T) {
	f := newFixture(t)
	if offers(t, f, "propose_project") {
		t.Error("a bot that is not the coordinator is offered propose_project")
	}
	f.coordinate()
	if !offers(t, f, "propose_project") {
		t.Error("the coordinator is not offered propose_project")
	}
	// A different bot answering for the org leaves this one without it.
	f.svc.d.Coordinator = func(context.Context) (org.Bot, bool) { return org.Bot{ID: "someone-else"}, true }
	if offers(t, f, "propose_project") {
		t.Error("a bot is offered propose_project while another is the coordinator")
	}
	f.coordinate()
	stored, err := f.db.GrantsFor(context.Background(), f.bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(stored, func(g org.Grant) bool { return g.Capability == project.CanPropose }) {
		t.Errorf("the run's grant was stored: %+v", stored)
	}
}

func TestProposeProject(t *testing.T) {
	f := newFixture(t,
		events(toolCall("c1", "propose_project", proposeArgs)),
		events(toolCall("c2", "propose_project", proposeArgs)),
		events(llm.StreamEvent{Token: "Proposed."}, llm.StreamEvent{Done: true}),
	)
	f.coordinate()
	_, evs := f.send(t, "Build me a launch page and a campaign.", ended)

	var changed []ProjectChanged
	for _, e := range evs {
		if pc, ok := e.Body.(ProjectChanged); ok {
			changed = append(changed, pc)
		}
	}
	if len(changed) != 1 {
		t.Fatalf("ProjectChanged events = %d, want 1 (the repeat is refused)", len(changed))
	}
	all, err := f.db.Projects(context.Background(), false)
	if err != nil || len(all) != 1 {
		t.Fatalf("projects = %+v, %v, want one", all, err)
	}
	p := all[0]
	if p.State != project.Proposed || p.Name != "Launch page" || p.ProposedByBotID != f.bot.ID || p.ProposedInConversationID != f.bot.DMConversationID {
		t.Errorf("stored = %+v, want a proposal of the bot in its DM", p)
	}
	if p.FolderPath != "" || p.ConversationID != "" {
		t.Errorf("a proposal has a folder %q or conversation %q", p.FolderPath, p.ConversationID)
	}
	if changed[0].Project.ID != p.ID {
		t.Errorf("event project = %s, want %s", changed[0].Project.ID, p.ID)
	}
	var refused bool
	for _, m := range f.messages(t) {
		if m.Kind == org.MessageToolResult && m.ToolCallID == "c2" && strings.Contains(m.Content, "already proposed") {
			refused = true
		}
	}
	if !refused {
		t.Error("the repeated proposal was not refused with already proposed")
	}
}
