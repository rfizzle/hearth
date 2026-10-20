package staff

import (
	"context"
	"time"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/draft/concern"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/workspace"
)

// The handlers hand their work to a method that returns the raw error;
// wire.Reply gives it its code, so no handler inspects one.

// ListProjects is every project with its status; archived ones only when
// asked for.
func (s *projectService) ListProjects(ctx context.Context, req *connect.Request[hearthv1.ListProjectsRequest]) (*connect.Response[hearthv1.ListProjectsResponse], error) {
	list, err := s.list(ctx, req.Msg.GetIncludeArchived())
	return wire.Reply(&hearthv1.ListProjectsResponse{Projects: list}, err)
}

func (s *projectService) list(ctx context.Context, includeArchived bool) ([]*hearthv1.Project, error) {
	ps, err := s.opts.Store.Projects(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]*hearthv1.Project, 0, len(ps))
	for _, p := range ps {
		st, err := s.opts.Orchestrator.Status(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, projectToProto(p, st))
	}
	return out, nil
}

// DraftProject asks the model, at the Fast tier, to turn the owner's
// paragraph into a project's fields. It stores nothing: the app shows the
// proposal and sends CreateProject with whatever the owner settles on. The
// paragraph is bounded here, before it goes anywhere; the concerns are the
// rules' first, then the model's, at most three.
// See docs/model.md#owner-interaction
func (s *projectService) DraftProject(ctx context.Context, req *connect.Request[hearthv1.DraftProjectRequest]) (*connect.Response[hearthv1.DraftProjectResponse], error) {
	description := req.Msg.GetDescription()
	if err := askable(description, maxDescription, s.opts.ProjectDrafter != nil); err != nil {
		return nil, err
	}
	d, err := s.opts.ProjectDrafter(ctx, description)
	var concerns []concern.Concern
	if err == nil {
		concerns = concern.Cap(concern.ForProject, concern.Project(d.Name, d.Brief, d.TargetDate, time.Now()), d.Concerns)
	}
	return wire.Reply(&hearthv1.DraftProjectResponse{
		Name: d.Name, Brief: briefToProto(d.Brief), TargetDate: d.TargetDate,
		Concerns: wire.Each(concerns, wire.ConcernToProto),
	}, err)
}

// CheckProject says what looks weak about a project's fields, by rule and
// with no model. It reads nothing and stores nothing.
// See docs/model.md#owner-interaction
func (s *projectService) CheckProject(_ context.Context, req *connect.Request[hearthv1.CheckProjectRequest]) (*connect.Response[hearthv1.CheckProjectResponse], error) {
	m := req.Msg
	rules := concern.Project(m.GetName(), briefFromProto(m.GetBrief()), m.GetTargetDate(), time.Now())
	return wire.Reply(&hearthv1.CheckProjectResponse{Concerns: wire.Each(concern.Cap(concern.ForProject, rules, nil), wire.ConcernToProto)}, nil)
}

// CreateProject makes an active project, or confirms the proposal named by
// proposal_id with the request's fields as its edits. The folder is made
// before the row is written, so a folder that cannot be made is refused
// before the project points at it.
func (s *projectService) CreateProject(ctx context.Context, req *connect.Request[hearthv1.CreateProjectRequest]) (*connect.Response[hearthv1.CreateProjectResponse], error) {
	created, err := s.create(ctx, req.Msg)
	return wire.Reply(&hearthv1.CreateProjectResponse{Project: created}, err)
}

func (s *projectService) create(ctx context.Context, m *hearthv1.CreateProjectRequest) (*hearthv1.Project, error) {
	o, err := s.opts.Store.Org(ctx)
	if err != nil {
		return nil, err
	}
	var p project.Project
	var conv org.Conversation
	if id := m.GetProposalId(); id != "" {
		p, conv, err = s.confirm(ctx, id, m, o.WorkspacePath)
	} else {
		p, conv, err = project.New(m.GetName(), briefFromProto(m.GetBrief()), m.GetTargetDate(), o.WorkspacePath)
	}
	if err != nil {
		return nil, err
	}
	if err := workspace.EnsureFolder(o.WorkspacePath, p.FolderPath); err != nil {
		return nil, wire.Invalid("%v", err)
	}
	if p.ID != "" {
		p, err = s.opts.Store.ActivateProject(ctx, p, conv)
	} else {
		p, err = s.opts.Store.CreateProject(ctx, p, conv)
	}
	return s.announce(ctx, p, err)
}

// confirm is the proposal id activated with the request's edits.
func (s *projectService) confirm(ctx context.Context, id string, m *hearthv1.CreateProjectRequest, root string) (project.Project, org.Conversation, error) {
	was, err := s.opts.Store.Project(ctx, id)
	if err != nil {
		return project.Project{}, org.Conversation{}, err
	}
	return project.Activate(was, m.GetName(), briefFromProto(m.GetBrief()), m.GetTargetDate(), root)
}

// UpdateProject replaces the name, the brief and the target date. The folder
// keeps its name.
func (s *projectService) UpdateProject(ctx context.Context, req *connect.Request[hearthv1.UpdateProjectRequest]) (*connect.Response[hearthv1.UpdateProjectResponse], error) {
	m := req.Msg
	p, err := s.opts.Store.EditProject(ctx, m.GetId(), m.GetName(), briefFromProto(m.GetBrief()), m.GetTargetDate())
	updated, err := s.announce(ctx, p, err)
	return wire.Reply(&hearthv1.UpdateProjectResponse{Project: updated}, err)
}

// MarkProjectDone is the owner's mark, from an active project only.
func (s *projectService) MarkProjectDone(ctx context.Context, req *connect.Request[hearthv1.MarkProjectDoneRequest]) (*connect.Response[hearthv1.MarkProjectDoneResponse], error) {
	return moved(ctx, s, req.Msg.GetId(), project.Done, func(p *hearthv1.Project) *hearthv1.MarkProjectDoneResponse {
		return &hearthv1.MarkProjectDoneResponse{Project: p}
	})
}

// moved takes the project id to state next if its state allows it and wraps
// the project as the RPC answers.
func moved[T any](ctx context.Context, s *projectService, id string, next project.State, wrap func(*hearthv1.Project) *T) (*connect.Response[T], error) {
	p, err := s.opts.Store.MoveProject(ctx, id, next, time.Now())
	out, err := s.announce(ctx, p, err)
	if err != nil {
		return nil, wire.Fail(err)
	}
	return connect.NewResponse(wrap(out)), nil
}

// announce tells the bus about p, as the store has it after the change that
// came with err, with its status, and returns it in the contract's form.
func (s *projectService) announce(ctx context.Context, p project.Project, err error) (*hearthv1.Project, error) {
	if err != nil {
		return nil, err
	}
	st, err := s.opts.Orchestrator.Status(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	s.opts.Publish(orchestrate.ProjectChanged{Project: p, Status: st})
	return projectToProto(p, st), nil
}

// ArchiveProject keeps the folder and the conversation; on a proposal it is
// "Not now".
func (s *projectService) ArchiveProject(ctx context.Context, req *connect.Request[hearthv1.ArchiveProjectRequest]) (*connect.Response[hearthv1.ArchiveProjectResponse], error) {
	p, err := s.opts.Store.MoveProject(ctx, req.Msg.GetId(), project.Archived, time.Now())
	archived, err := s.announce(ctx, p, err)
	return wire.Reply(&hearthv1.ArchiveProjectResponse{Project: archived}, err)
}
