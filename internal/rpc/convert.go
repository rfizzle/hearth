package rpc

import (
	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
)

// The conversions between org values and proto messages. An enum that org
// and the contract number alike is a cast, and a test holds the two lists
// together; the ones that differ in kind (a provider's name, an
// integration's name) go through a table.

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

// integrationNames is the contract's integration enum against integration's
// names.
var integrationNames = map[hearthv1.Integration]integration.Name{
	hearthv1.Integration_INTEGRATION_BRAVE_SEARCH: integration.BraveSearch,
}

var accountStates = map[provider.State]hearthv1.AccountState{
	provider.SignedOut: hearthv1.AccountState_ACCOUNT_STATE_SIGNED_OUT,
	provider.SigningIn: hearthv1.AccountState_ACCOUNT_STATE_SIGNING_IN,
	provider.SignedIn:  hearthv1.AccountState_ACCOUNT_STATE_SIGNED_IN,
}

// accountToProto carries a label and a failure, never a credential: the
// provider's Account holds none.
func accountToProto(a provider.Account) *hearthv1.ProviderAccount {
	return &hearthv1.ProviderAccount{
		Provider:     wire.ToProto(wire.ProviderNames, a.Provider),
		State:        accountStates[a.State],
		Label:        a.Label,
		Failure:      a.Failure,
		Profile:      a.Profile,
		DisplayName:  a.DisplayName,
		BaseUrl:      a.BaseURL,
		DefaultModel: a.DefaultModel,
	}
}

// integrationAccountToProto carries the label (the last four characters)
// and never the key, like the provider's.
func integrationAccountToProto(a integration.Account) *hearthv1.IntegrationAccount {
	kind := hearthv1.IntegrationKind_INTEGRATION_KIND_UNSPECIFIED
	if a.Kind == integration.WebSearch {
		kind = hearthv1.IntegrationKind_INTEGRATION_KIND_WEB_SEARCH
	}
	return &hearthv1.IntegrationAccount{
		Integration: wire.ToProto(integrationNames, a.Name),
		Kind:        kind,
		Name:        a.Title,
		Configured:  a.Configured,
		Label:       a.Label,
		HelpUrl:     a.HelpURL,
	}
}
