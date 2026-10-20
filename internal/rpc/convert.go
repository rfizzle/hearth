package rpc

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rfizzle/hearth/internal/integration"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/widget"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// The conversions between org values and proto messages. An enum that org
// and the contract number alike is a cast, and a test holds the two lists
// together; the ones that differ in kind (a provider's name, an
// integration's name) go through a table.

func timestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func tierFromProto(t hearthv1.Tier) (org.Tier, error) {
	switch t {
	case hearthv1.Tier_TIER_FAST, hearthv1.Tier_TIER_BALANCED, hearthv1.Tier_TIER_BEST:
		return org.Tier(t), nil
	}
	return 0, invalid("tier is required")
}

func botToProto(b org.Bot) *hearthv1.Bot {
	return &hearthv1.Bot{
		Id:               b.ID,
		Name:             b.Name,
		Role:             b.Role,
		JobDescription:   b.JobDescription,
		Tier:             hearthv1.Tier(b.Tier),
		Status:           hearthv1.BotStatus(b.Status),
		Avatar:           b.Avatar,
		FolderPath:       b.FolderPath,
		DmConversationId: b.DMConversationID,
	}
}

func templateToProto(t org.RoleTemplate) *hearthv1.RoleTemplate {
	return &hearthv1.RoleTemplate{
		Id:             t.ID,
		Role:           t.Role,
		Summary:        t.Summary,
		JobDescription: t.JobDescription,
		SuggestedTier:  hearthv1.Tier(t.SuggestedTier),
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
		UpdatedAt: timestamp(c.UpdatedAt),
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
		CreatedAt:      timestamp(m.CreatedAt),
		RunId:          m.RunID,
		Streaming:      m.Streaming,
	}
	var spec widget.Spec
	out.Widget, spec = widgetOf(m.Widget)
	out.Answer = answerOf(m.WidgetAnswer, spec)
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
		StartedAt:      timestamp(r.StartedAt),
		EndedAt:        timestamp(r.EndedAt),
	}
}

// providerNames is the contract's provider enum against provider's names.
var providerNames = map[hearthv1.Provider]provider.Provider{
	hearthv1.Provider_PROVIDER_CHATGPT:           provider.ChatGPT,
	hearthv1.Provider_PROVIDER_ANTHROPIC:         provider.Anthropic,
	hearthv1.Provider_PROVIDER_GEMINI:            provider.Gemini,
	hearthv1.Provider_PROVIDER_OPENROUTER:        provider.OpenRouter,
	hearthv1.Provider_PROVIDER_OPENAI_COMPATIBLE: provider.Compatible,
}

// knownKinds is the contract's known-service kinds against provider's.
var knownKinds = map[hearthv1.KnownServiceKind]provider.KnownKind{
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_BUILTIN:    provider.Builtin,
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_COMPATIBLE: provider.CompatibleService,
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_LOCAL:      provider.Local,
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

// fromProto is the domain name for a contract enum, or invalid_argument when
// hearth has none for it (unspecified, or not built yet).
func fromProto[E interface {
	comparable
	fmt.Stringer
}, N any](names map[E]N, e E) (N, error) {
	n, ok := names[e]
	if !ok {
		return n, invalid("%s is not one hearth can use", e)
	}
	return n, nil
}

// toProto is the contract enum for a domain name; the zero value, which the
// contract reads as unspecified, when there is none.
func toProto[E, N comparable](names map[E]N, n N) E {
	for e, name := range names {
		if name == n {
			return e
		}
	}
	var none E
	return none
}

// accountToProto carries a label and a failure, never a credential: the
// provider's Account holds none.
func accountToProto(a provider.Account) *hearthv1.ProviderAccount {
	return &hearthv1.ProviderAccount{
		Provider:     toProto(providerNames, a.Provider),
		State:        accountStates[a.State],
		Label:        a.Label,
		Failure:      a.Failure,
		Profile:      a.Profile,
		DisplayName:  a.DisplayName,
		BaseUrl:      a.BaseURL,
		DefaultModel: a.DefaultModel,
	}
}

func knownServiceToProto(k provider.KnownService) *hearthv1.KnownService {
	return &hearthv1.KnownService{
		Id:           k.ID,
		Name:         k.Name,
		BaseUrl:      k.BaseURL,
		HelpUrl:      k.HelpURL,
		DefaultModel: k.DefaultModel,
		Kind:         toProto(knownKinds, k.Kind),
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
		Integration: toProto(integrationNames, a.Name),
		Kind:        kind,
		Name:        a.Title,
		Configured:  a.Configured,
		Label:       a.Label,
		HelpUrl:     a.HelpURL,
	}
}
