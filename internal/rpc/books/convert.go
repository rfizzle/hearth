package books

import (
	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/provider/models"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/store"
)

// The conversions between the notes, pages, totals and tiers and the
// contract's messages.

func noteToProto(n memory.Note) *hearthv1.Note {
	return &hearthv1.Note{
		Id: n.ID, BotId: n.BotID, Text: n.Text, Pinned: n.Pinned,
		ConversationId: n.ConversationID, MessageId: n.MessageID, CreatedAt: wire.Timestamp(n.CreatedAt),
	}
}

// pageToProto carries the page's size as the meter counts it.
func pageToProto(p memory.Page) *hearthv1.HandbookPage {
	size, _ := memory.Meter([]memory.Page{p})
	scope := &hearthv1.Scope{Everyone: p.Scope.Everyone, BotId: p.Scope.BotID}
	return &hearthv1.HandbookPage{Id: p.ID, Title: p.Title, Markdown: p.Markdown, Scope: scope, Bytes: size, UpdatedAt: wire.Timestamp(p.UpdatedAt)}
}

func meterToProto(m handbook.Meter) *hearthv1.ContextMeter {
	return &hearthv1.ContextMeter{UsedBytes: m.Used, CapBytes: m.Cap}
}

// totalsToProto is priced when every request was priced or on a plan, so
// the app shows a cost only when none is missing.
func totalsToProto(t usage.Totals) *hearthv1.UsageTotals {
	return &hearthv1.UsageTotals{
		Requests: t.Requests, PromptTokens: t.Prompt, CompletionTokens: t.Completion, CachedTokens: t.Cached,
		CostMicros: t.CostMicros, Priced: t.Unpriced == 0,
	}
}

func botUsageToProto(b meter.BotSummary) *hearthv1.BotUsage {
	return &hearthv1.BotUsage{BotId: b.BotID, Totals: totalsToProto(b.Totals), BudgetCents: b.Budget.Cents, Paused: b.Paused}
}

func summaryToProto(s meter.Summary) *hearthv1.SummaryResponse {
	return &hearthv1.SummaryResponse{
		Month: s.Month, Org: totalsToProto(s.Org), Bots: wire.Each(s.Bots, botUsageToProto),
		OrgBudgetCents: s.OrgBudget.Cents, OrgPaused: s.OrgPaused, Months: s.Months,
	}
}

func tierToProto(c store.TierChoice) *hearthv1.TierChoice {
	p := wire.ToProto(wire.ProviderNames, provider.Provider(c.Provider))
	return &hearthv1.TierChoice{Tier: hearthv1.Tier(c.Tier), Provider: p, Profile: c.Profile, Model: c.Model}
}

// tierFromProto is one row of the owner's mapping; the tier, the provider
// and the model must each be named.
func tierFromProto(c *hearthv1.TierChoice) (store.TierChoice, error) {
	tier, err := wire.TierFromProto(c.GetTier())
	if err != nil {
		return store.TierChoice{}, err
	}
	p, err := wire.FromProto(wire.ProviderNames, c.GetProvider())
	if err != nil {
		return store.TierChoice{}, err
	}
	return store.TierChoice{Tier: tier, Provider: string(p), Profile: c.GetProfile(), Model: c.GetModel()}, wire.Need("model", c.GetModel())
}

func optionToProto(o models.Option) *hearthv1.ModelOption {
	p := wire.ToProto(wire.ProviderNames, o.Pick.Provider)
	return &hearthv1.ModelOption{Provider: p, Profile: o.Pick.Profile, Model: o.Pick.Model, Label: o.Label}
}

// knownKinds is the contract's known-service kinds against provider's.
var knownKinds = map[hearthv1.KnownServiceKind]provider.KnownKind{
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_BUILTIN:    provider.Builtin,
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_COMPATIBLE: provider.CompatibleService,
	hearthv1.KnownServiceKind_KNOWN_SERVICE_KIND_LOCAL:      provider.Local,
}

// knownServiceToProto is a known service with what detection found.
func knownServiceToProto(k models.Service) *hearthv1.KnownService {
	return &hearthv1.KnownService{
		Id: k.ID, Name: k.Name, BaseUrl: k.BaseURL, HelpUrl: k.HelpURL, DefaultModel: k.DefaultModel,
		Kind: wire.ToProto(knownKinds, k.Kind), Detected: k.Detected, Models: k.Models,
	}
}
