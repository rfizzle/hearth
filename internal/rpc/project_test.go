package rpc

import (
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

var launchBrief = &hearthv1.ProjectBrief{
	Goal: "Launch the shop", Context: "We sell mugs", DoneLooksLike: "A live page", DoneWhen: "Orders arrive",
}

// onboard names the org and returns the Chief of Staff's id.
func onboard(t *testing.T, c clients, r *rig) string {
	t.Helper()
	unary(t, c.org.UpdateOrg, &hearthv1.UpdateOrgRequest{Name: "Acme", WorkspacePath: r.root})
	return unary(t, c.bot.ListBots, &hearthv1.ListBotsRequest{}).GetBots()[0].GetId()
}

func TestCreateProject_FolderConversationAndEvent(t *testing.T) {
	r := newCoordinatedRig(t, answer("On it."))
	c := newClients(serve(t, r.opts), testToken)
	chief := onboard(t, c, r)
	f := follow(t, c, r.bus)

	p := unary(t, c.proj.CreateProject, &hearthv1.CreateProjectRequest{Name: " Launch ", Brief: launchBrief, TargetDate: "2026-12-31"}).GetProject()
	folder := filepath.Join(r.root, "Projects", "Launch")
	if p.GetName() != "Launch" || p.GetState() != hearthv1.ProjectState_PROJECT_STATE_ACTIVE || p.GetFolderPath() != folder ||
		p.GetConversationId() == "" || p.GetStatus().GetLight() != hearthv1.ProjectLight_PROJECT_LIGHT_NONE ||
		p.GetStatus().GetLightLabel() != "No tasks yet" || p.GetTargetDate() != "2026-12-31" {
		t.Fatalf("CreateProject = %v", p)
	}
	if fi, err := os.Stat(folder); err != nil || !fi.IsDir() {
		t.Fatalf("the project folder was not made: %v", err)
	}
	evs := f.until(func(m *hearthv1.SubscribeResponse) bool { return m.GetProjectChanged() != nil })
	if got := evs[len(evs)-1].GetProjectChanged().GetProject(); got.GetId() != p.GetId() || got.GetStatus() == nil {
		t.Fatalf("project_changed = %v, want the new project with its status", got)
	}

	conv := unary(t, c.conv.GetConversation, &hearthv1.GetConversationRequest{Id: p.GetConversationId()}).GetConversation()
	if conv.GetKind() != hearthv1.ConversationKind_CONVERSATION_KIND_PROJECT || conv.GetProjectId() != p.GetId() {
		t.Fatalf("GetConversation = %v, want the project's", conv)
	}
	if got := unary(t, c.org.GetOrg, &hearthv1.GetOrgRequest{}).GetOrg().GetCoordinatorBotId(); got != chief {
		t.Fatalf("coordinator_bot_id = %q, want %q", got, chief)
	}

	sent := unary(t, c.conv.Send, &hearthv1.SendRequest{ConversationId: p.GetConversationId(), Text: "Begin"})
	f.until(runEnded)
	page := unary(t, c.conv.ListMessages, &hearthv1.ListMessagesRequest{ConversationId: p.GetConversationId()})
	if sent.GetRun().GetId() == "" || len(page.GetMessages()) != 2 || page.GetMessages()[1].GetAuthorBotId() != chief {
		t.Fatalf("after Send the project conversation holds %v, want the coordinator's reply", page.GetMessages())
	}

	list := unary(t, c.proj.ListProjects, &hearthv1.ListProjectsRequest{}).GetProjects()
	if len(list) != 1 || list[0].GetId() != p.GetId() || list[0].GetStatus().GetLightLabel() != "No tasks yet" {
		t.Fatalf("ListProjects = %v", list)
	}
}

func TestCreateProject_ConfirmingAProposalKeepsItsID(t *testing.T) {
	r := newCoordinatedRig(t)
	c := newClients(serve(t, r.opts), testToken)
	chief := onboard(t, c, r)
	dm := unary(t, c.bot.GetBot, &hearthv1.GetBotRequest{Id: chief}).GetBot().GetDmConversationId()
	proposal, err := project.Propose("Launch", project.Brief{Goal: "g", Context: "c", DoneLooksLike: "d", DoneWhen: "w"}, chief, dm)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := r.opts.Store.CreateProject(t.Context(), proposal, org.Conversation{})
	if err != nil {
		t.Fatal(err)
	}

	p := unary(t, c.proj.CreateProject, &hearthv1.CreateProjectRequest{Name: "Launch two", Brief: launchBrief, ProposalId: stored.ID}).GetProject()
	if p.GetId() != stored.ID || p.GetName() != "Launch two" || p.GetState() != hearthv1.ProjectState_PROJECT_STATE_ACTIVE ||
		p.GetProposedByBotId() != chief || p.GetConversationId() == "" || p.GetBrief().GetGoal() != "Launch the shop" {
		t.Fatalf("confirmed = %v", p)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Projects", "Launch two")); err != nil {
		t.Fatalf("the folder was not made: %v", err)
	}
	_, err = c.proj.CreateProject(t.Context(), connect.NewRequest(&hearthv1.CreateProjectRequest{Name: "Again", Brief: launchBrief, ProposalId: stored.ID}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("confirming an active project = %v, want failed_precondition", err)
	}
}

func TestProjectLifecycle(t *testing.T) {
	r := newCoordinatedRig(t)
	c := newClients(serve(t, r.opts), testToken)
	onboard(t, c, r)
	p := unary(t, c.proj.CreateProject, &hearthv1.CreateProjectRequest{Name: "Launch", Brief: launchBrief}).GetProject()

	up := unary(t, c.proj.UpdateProject, &hearthv1.UpdateProjectRequest{Id: p.GetId(), Name: "Relaunch", Brief: launchBrief, TargetDate: "2027-01-02"}).GetProject()
	if up.GetName() != "Relaunch" || up.GetTargetDate() != "2027-01-02" || up.GetFolderPath() != p.GetFolderPath() {
		t.Fatalf("UpdateProject = %v, want the folder kept", up)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Projects", "Relaunch")); !os.IsNotExist(err) {
		t.Fatalf("a rename made or moved a folder: %v", err)
	}

	done := unary(t, c.proj.MarkProjectDone, &hearthv1.MarkProjectDoneRequest{Id: p.GetId()}).GetProject()
	if done.GetState() != hearthv1.ProjectState_PROJECT_STATE_DONE || done.GetDoneAt() == nil {
		t.Fatalf("MarkProjectDone = %v", done)
	}
	_, err := c.proj.MarkProjectDone(t.Context(), connect.NewRequest(&hearthv1.MarkProjectDoneRequest{Id: p.GetId()}))
	if code(err) != connect.CodeFailedPrecondition {
		t.Fatalf("done twice = %v, want failed_precondition", err)
	}
	arch := unary(t, c.proj.ArchiveProject, &hearthv1.ArchiveProjectRequest{Id: p.GetId()}).GetProject()
	if arch.GetState() != hearthv1.ProjectState_PROJECT_STATE_ARCHIVED || arch.GetArchivedAt() == nil || arch.GetConversationId() != p.GetConversationId() {
		t.Fatalf("ArchiveProject = %v", arch)
	}
	for name, call := range map[string]func() error{
		"done on an archived project": func() error {
			_, err := c.proj.MarkProjectDone(t.Context(), connect.NewRequest(&hearthv1.MarkProjectDoneRequest{Id: p.GetId()}))
			return err
		},
		"archive twice": func() error {
			_, err := c.proj.ArchiveProject(t.Context(), connect.NewRequest(&hearthv1.ArchiveProjectRequest{Id: p.GetId()}))
			return err
		},
	} {
		if got := code(call()); got != connect.CodeFailedPrecondition {
			t.Errorf("%s = %v, want failed_precondition", name, got)
		}
	}
	if n := len(unary(t, c.proj.ListProjects, &hearthv1.ListProjectsRequest{}).GetProjects()); n != 0 {
		t.Errorf("ListProjects listed %d archived", n)
	}
	if n := len(unary(t, c.proj.ListProjects, &hearthv1.ListProjectsRequest{IncludeArchived: true}).GetProjects()); n != 1 {
		t.Errorf("ListProjects with archived listed %d, want 1", n)
	}
}

func TestProjectErrorsCarryTheCodeTheAppReads(t *testing.T) {
	r := newRig(t)
	c := newClients(serve(t, r.opts), testToken)
	onboard(t, c, r)
	p := unary(t, c.proj.CreateProject, &hearthv1.CreateProjectRequest{Name: "Launch", Brief: launchBrief}).GetProject()
	ctx := t.Context()
	tests := []struct {
		name string
		call func() error
		want connect.Code
	}{
		{"create with no brief", func() error {
			_, err := c.proj.CreateProject(ctx, connect.NewRequest(&hearthv1.CreateProjectRequest{Name: "X"}))
			return err
		}, connect.CodeInvalidArgument},
		{"create with a bad date", func() error {
			_, err := c.proj.CreateProject(ctx, connect.NewRequest(&hearthv1.CreateProjectRequest{Name: "X", Brief: launchBrief, TargetDate: "soon"}))
			return err
		}, connect.CodeInvalidArgument},
		{"create into a folder a project has", func() error {
			_, err := c.proj.CreateProject(ctx, connect.NewRequest(&hearthv1.CreateProjectRequest{Name: "launch", Brief: launchBrief}))
			return err
		}, connect.CodeAlreadyExists},
		{"update with no name", func() error {
			_, err := c.proj.UpdateProject(ctx, connect.NewRequest(&hearthv1.UpdateProjectRequest{Id: p.GetId(), Brief: launchBrief}))
			return err
		}, connect.CodeInvalidArgument},
		{"update an unknown project", func() error {
			_, err := c.proj.UpdateProject(ctx, connect.NewRequest(&hearthv1.UpdateProjectRequest{Id: "nope", Name: "X", Brief: launchBrief}))
			return err
		}, connect.CodeNotFound},
		{"confirm an unknown proposal", func() error {
			_, err := c.proj.CreateProject(ctx, connect.NewRequest(&hearthv1.CreateProjectRequest{Name: "X", Brief: launchBrief, ProposalId: "nope"}))
			return err
		}, connect.CodeNotFound},
		{"archive an unknown project", func() error {
			_, err := c.proj.ArchiveProject(ctx, connect.NewRequest(&hearthv1.ArchiveProjectRequest{Id: "nope"}))
			return err
		}, connect.CodeNotFound},
		{"send with no coordinator", func() error {
			_, err := c.conv.Send(ctx, connect.NewRequest(&hearthv1.SendRequest{ConversationId: p.GetConversationId(), Text: "hi"}))
			return err
		}, connect.CodeFailedPrecondition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := code(tt.call()); got != tt.want {
				t.Errorf("code = %v, want %v", got, tt.want)
			}
		})
	}
	if got := unary(t, c.org.GetOrg, &hearthv1.GetOrgRequest{}).GetOrg().GetCoordinatorBotId(); got != "" {
		t.Errorf("coordinator_bot_id = %q with none wired, want empty", got)
	}
}
