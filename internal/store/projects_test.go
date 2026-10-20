package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
)

var brief = project.Brief{Goal: "Launch", Context: "Mugs", DoneLooksLike: "A page", DoneWhen: "Orders"}

func newProject(t *testing.T, name string) (project.Project, org.Conversation) {
	t.Helper()
	p, conv, err := project.New(name, brief, "2026-12-31", orgFixture.WorkspacePath)
	if err != nil {
		t.Fatal(err)
	}
	return p, conv
}

func TestCreateProject_MakesItsConversation(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	p, conv := newProject(t, "Launch")
	got, err := db.CreateProject(ctx, p, conv)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.ConversationID == "" || got.State != project.Active || got.ActivatedAt.IsZero() {
		t.Fatalf("created = %+v", got)
	}
	read, err := db.Project(ctx, got.ID)
	if err != nil || read != got {
		t.Fatalf("Project = %+v, %v, want %+v", read, err, got)
	}
	c, err := db.Conversation(ctx, got.ConversationID)
	if err != nil || c.Kind != org.ConversationProject || c.ProjectID != got.ID || c.BotID != "" || c.Title != "Launch" {
		t.Fatalf("Conversation = %+v, %v", c, err)
	}
	dana := hire(t, db, "Dana")
	dm, err := db.Conversation(ctx, dana.DMConversationID)
	if err != nil || dm.ProjectID != "" || dm.Kind != org.ConversationDM {
		t.Fatalf("DM = %+v, %v, want no project id", dm, err)
	}
	if _, err := db.Project(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Project of a missing id: %v", err)
	}
}

func TestProposeThenActivate_KeepsTheIDAndMakesTheConversation(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	dana := hire(t, db, "Dana")
	proposal, err := project.Propose("Launch", brief, dana.ID, dana.DMConversationID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := db.CreateProject(ctx, proposal, org.Conversation{})
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != project.Proposed || stored.ConversationID != "" || stored.FolderPath != "" ||
		stored.ProposedByBotID != dana.ID || stored.ProposedInConversationID != dana.DMConversationID {
		t.Fatalf("proposal = %+v", stored)
	}
	active, conv, err := project.Activate(stored, "Launch two", brief, "", orgFixture.WorkspacePath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.ActivateProject(ctx, active, conv)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != stored.ID || got.State != project.Active || got.Name != "Launch two" || got.ConversationID == "" ||
		got.ProposedByBotID != dana.ID || got.FolderPath != active.FolderPath || got.ActivatedAt.IsZero() {
		t.Fatalf("activated = %+v", got)
	}
	c, err := db.Conversation(ctx, got.ConversationID)
	if err != nil || c.ProjectID != got.ID || c.Kind != org.ConversationProject {
		t.Fatalf("Conversation = %+v, %v", c, err)
	}
	// A project already active is not a proposal to activate again.
	if _, err := db.ActivateProject(ctx, active, conv); err == nil {
		t.Fatal("a second ActivateProject was accepted")
	}
}

func TestCreateProject_FolderNamesDifferingInCaseAreTaken(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	p, conv := newProject(t, "Launch")
	if _, err := db.CreateProject(ctx, p, conv); err != nil {
		t.Fatal(err)
	}
	q, conv := newProject(t, "LAUNCH")
	if _, err := db.CreateProject(ctx, q, conv); !errors.Is(err, ErrFolderTaken) {
		t.Fatalf("CreateProject = %v, want ErrFolderTaken", err)
	}
	if got, err := db.Projects(ctx, true); err != nil || len(got) != 1 {
		t.Fatalf("Projects = %d, %v, want the first only", len(got), err)
	}
}

func TestArchiveKeepsTheRowAndTheConversation(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	a, conv := newProject(t, "Alpha")
	a, err := db.CreateProject(ctx, a, conv)
	if err != nil {
		t.Fatal(err)
	}
	b, conv := newProject(t, "Beta")
	b, err = db.CreateProject(ctx, b, conv)
	if err != nil {
		t.Fatal(err)
	}
	at := time.UnixMilli(1_700_000_000_000)
	if err := db.SetProjectState(ctx, a.ID, project.Archived, at); err != nil {
		t.Fatal(err)
	}
	got, err := db.Project(ctx, a.ID)
	if err != nil || got.State != project.Archived || !got.ArchivedAt.Equal(at) || got.ConversationID != a.ConversationID {
		t.Fatalf("archived = %+v, %v", got, err)
	}
	if _, err := db.Conversation(ctx, a.ConversationID); err != nil {
		t.Fatalf("the conversation after archive: %v", err)
	}
	live, err := db.Projects(ctx, false)
	if err != nil || len(live) != 1 || live[0].ID != b.ID {
		t.Fatalf("Projects(false) = %+v, %v, want only Beta", live, err)
	}
	all, err := db.Projects(ctx, true)
	if err != nil || len(all) != 2 || all[0].ID != a.ID {
		t.Fatalf("Projects(true) = %+v, %v, want both, oldest first", all, err)
	}
	if err := db.SetProjectState(ctx, b.ID, project.Done, at); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Project(ctx, b.ID); got.State != project.Done || !got.DoneAt.Equal(at) {
		t.Fatalf("done = %+v", got)
	}
	if err := db.SetProjectState(ctx, "nope", project.Done, at); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetProjectState of a missing id: %v", err)
	}
}

func TestUpdateProject_NeverMovesTheFolder(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	p, conv := newProject(t, "Launch")
	p, err := db.CreateProject(ctx, p, conv)
	if err != nil {
		t.Fatal(err)
	}
	edited := p
	edited.Name, edited.TargetDate, edited.Brief.Goal = "Renamed", "2027-01-01", "Sell more"
	edited.FolderPath = "/elsewhere"
	if err := db.UpdateProject(ctx, edited); err != nil {
		t.Fatal(err)
	}
	got, err := db.Project(ctx, p.ID)
	if err != nil || got.Name != "Renamed" || got.TargetDate != "2027-01-01" || got.Brief.Goal != "Sell more" || got.FolderPath != p.FolderPath {
		t.Fatalf("updated = %+v, %v", got, err)
	}
	if err := db.UpdateProject(ctx, project.Project{ID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateProject of a missing id: %v", err)
	}
}

func TestProjects_SurviveAReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hearth.db")
	db := open(t, path)
	p, conv := newProject(t, "Launch")
	p, err := db.CreateProject(t.Context(), p, conv)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db = open(t, path)
	if got, err := db.Project(t.Context(), p.ID); err != nil || got != p {
		t.Fatalf("after reopening = %+v, %v, want %+v", got, err, p)
	}
}

func TestBotFolderTakenByAProject(t *testing.T) {
	db := newStore(t)
	p, conv := newProject(t, "Launch")
	if _, err := db.CreateProject(t.Context(), p, conv); err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Finds things out.", org.TierBalanced, orgFixture.WorkspacePath)
	bot.FolderPath = p.FolderPath
	if _, err := db.CreateBot(t.Context(), bot, grants, dm); !errors.Is(err, ErrFolderTaken) {
		t.Fatalf("CreateBot into a project's folder = %v, want ErrFolderTaken", err)
	}
}
