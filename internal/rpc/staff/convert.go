package staff

import (
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// The conversions between org values and proto messages. An enum that org
// and the contract number alike is a cast, and a test holds the two lists
// together; the ones that differ in kind go through a table.

func templateToProto(t org.RoleTemplate) *hearthv1.RoleTemplate {
	return &hearthv1.RoleTemplate{
		Id:             t.ID,
		Role:           t.Role,
		Summary:        t.Summary,
		JobDescription: t.JobDescription,
		SuggestedTier:  hearthv1.Tier(t.SuggestedTier),
		Introduction:   wire.IntroductionToProto(t.Introduction),
	}
}

func orgToProto(o org.Org) *hearthv1.Org {
	return &hearthv1.Org{Name: o.Name, WorkspacePath: o.WorkspacePath}
}

func conversationToProto(c org.Conversation) *hearthv1.Conversation {
	out := &hearthv1.Conversation{
		Id:        c.ID,
		Kind:      hearthv1.ConversationKind(c.Kind),
		BotId:     c.BotID,
		ProjectId: c.ProjectID,
		Title:     c.Title,
		UpdatedAt: wire.Timestamp(c.UpdatedAt),

		Summary:           c.Summary,
		SummaryThroughSeq: c.SummaryThroughSeq,
	}
	if c.LatestRun != nil {
		out.LatestRun = runToProto(*c.LatestRun)
	}
	return out
}

func messageToProto(m org.Message) *hearthv1.Message {
	out := &hearthv1.Message{
		Id:             m.ID,
		ConversationId: m.ConversationID,
		Seq:            m.Seq,
		AuthorBotId:    m.AuthorBotID,
		Kind:           hearthv1.MessageKind(m.Kind),
		Content:        m.Content,
		CreatedAt:      wire.Timestamp(m.CreatedAt),
		RunId:          m.RunID,
		Streaming:      m.Streaming,
	}
	control := wire.ControlOf(m.Widget)
	out.Widget, out.Answer = control.Proto(), control.AnswerProto(m.WidgetAnswer)
	// A tool message carries one call; the contract has room for one. A
	// result names the call it answers.
	switch {
	case len(m.ToolCalls) > 0:
		tc := m.ToolCalls[0]
		out.Tool = &hearthv1.ToolCall{CallId: tc.ID, Name: tc.Name, ArgumentsJson: tc.Arguments, Summary: tc.Summary}
		if m.ToolCallID != "" {
			out.Tool.CallId = m.ToolCallID
		}
	case m.ToolCallID != "":
		out.Tool = &hearthv1.ToolCall{CallId: m.ToolCallID}
	}
	for _, f := range m.Files {
		out.Files = append(out.Files, &hearthv1.FileRef{Path: f.Path, Name: f.Name})
	}
	return out
}

func runToProto(r org.Run) *hearthv1.Run {
	return &hearthv1.Run{
		Id:             r.ID,
		ConversationId: r.ConversationID,
		BotId:          r.BotID,
		State:          hearthv1.RunState(r.State),
		Steps:          int32(r.Steps),
		Failure:        r.Failure,
		StartedAt:      wire.Timestamp(r.StartedAt),
		EndedAt:        wire.Timestamp(r.EndedAt),
	}
}

// projectToProto is a project with its status; the states and lights are
// casts, held by a test.
func projectToProto(p project.Project, st project.Status) *hearthv1.Project {
	return &hearthv1.Project{
		Id:                       p.ID,
		Name:                     p.Name,
		Brief:                    &hearthv1.ProjectBrief{Goal: p.Brief.Goal, Context: p.Brief.Context, DoneLooksLike: p.Brief.DoneLooksLike, DoneWhen: p.Brief.DoneWhen},
		State:                    hearthv1.ProjectState(p.State),
		TargetDate:               p.TargetDate,
		FolderPath:               p.FolderPath,
		ConversationId:           p.ConversationID,
		Status:                   statusToProto(st),
		ProposedByBotId:          p.ProposedByBotID,
		ProposedInConversationId: p.ProposedInConversationID,
		CreatedAt:                wire.Timestamp(p.CreatedAt),
		ActivatedAt:              wire.Timestamp(p.ActivatedAt),
		DoneAt:                   wire.Timestamp(p.DoneAt),
		ArchivedAt:               wire.Timestamp(p.ArchivedAt),
		UpdatedAt:                wire.Timestamp(p.UpdatedAt),
	}
}

func statusToProto(st project.Status) *hearthv1.ProjectStatus {
	c := st.Counts
	return &hearthv1.ProjectStatus{
		TasksTotal:        int32(c.Total),
		TasksDone:         int32(c.Done),
		TasksWorking:      int32(c.Working),
		TasksWaitingOwner: int32(c.WaitingOwner),
		TasksPausedBudget: int32(c.PausedBudget),
		TasksFailed:       int32(c.Failed),
		TasksCancelled:    int32(c.Cancelled),
		Light:             hearthv1.ProjectLight(st.Light),
		LightLabel:        st.Label,
		AllTasksDone:      st.AllDone,
	}
}

// briefFromProto is the brief a request carries; none is the empty brief,
// which project refuses.
func briefFromProto(b *hearthv1.ProjectBrief) project.Brief {
	return project.Brief{Goal: b.GetGoal(), Context: b.GetContext(), DoneLooksLike: b.GetDoneLooksLike(), DoneWhen: b.GetDoneWhen()}
}
