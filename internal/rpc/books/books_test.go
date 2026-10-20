package books

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	storebooks "github.com/rfizzle/hearth/internal/store/books"

	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
)

// ledger is a meter that answers one summary.
type ledger struct {
	sum meter.Summary
	err error
}

func (l ledger) Summary(context.Context, string) (meter.Summary, error) { return l.sum, l.err }
func (ledger) SetBudgets(context.Context, []usage.Budget) error         { return nil }
func (ledger) Resume(context.Context, string) error                     { return nil }

func TestTicks_CarryTheBudgetFromTheSummary(t *testing.T) {
	row := meter.BotSummary{BotTotals: usage.BotTotals{BotID: "b1"}, Budget: usage.Budget{Subject: "b1", Cents: 2000}, Paused: true}
	tests := []struct {
		name   string
		l      ledger
		cents  int64
		paused bool
	}{
		{"read", ledger{sum: meter.Summary{Bots: []meter.BotSummary{row}}}, 2000, true},
		{"unreadable", ledger{err: errors.New("locked")}, 0, false},
	}
	for _, tt := range tests {
		var got []any
		Ticks(tt.l, func(b any) { got = append(got, b) })(meter.Tick{BotID: "b1", Month: "2026-10", Bot: usage.Totals{Prompt: 9}, Org: usage.Totals{Prompt: 12}})
		tick, ok := got[0].(UsageTicked)
		if len(got) != 1 || !ok || tick.Bot.Prompt != 9 || tick.Org.Prompt != 12 || tick.Bot.Budget.Cents != tt.cents || tick.Bot.Paused != tt.paused {
			t.Errorf("%s: published %+v", tt.name, got)
		}
	}
}

func TestEvent_ConvertsWhatBooksPublishes(t *testing.T) {
	bodies := []struct {
		body any
		has  func(*hearthv1.SubscribeResponse) bool
	}{
		{wire.NoteChanged{BotID: "b1", Note: memory.Note{ID: "n1"}, Deleted: true}, func(m *hearthv1.SubscribeResponse) bool {
			return m.GetNoteChanged().GetDeleted() && m.GetNoteChanged().GetBotId() == "b1"
		}},
		{PageChanged{Page: memory.Page{ID: "p1", Markdown: "abc"}}, func(m *hearthv1.SubscribeResponse) bool {
			return m.GetHandbookChanged().GetPage().GetBytes() == 3
		}},
		{TiersChanged{Tiers: []storebooks.TierChoice{{Tier: org.TierBest, Provider: "anthropic", Model: "m"}}}, func(m *hearthv1.SubscribeResponse) bool {
			c := m.GetTiersChanged().GetTiers()
			return len(c) == 1 && c[0].GetProvider() == hearthv1.Provider_PROVIDER_ANTHROPIC && c[0].GetTier() == hearthv1.Tier_TIER_BEST
		}},
		{UsageTicked{Month: "2026-10", Org: usage.Totals{Requests: 1, Unpriced: 1}}, func(m *hearthv1.SubscribeResponse) bool {
			return m.GetUsageTick().GetMonth() == "2026-10" && !m.GetUsageTick().GetOrg().GetPriced()
		}},
	}
	for _, b := range bodies {
		if m, ok := Event(b.body); !ok || !b.has(m) {
			t.Errorf("Event(%T) = %v, %v", b.body, m, ok)
		}
	}
	if _, ok := Event("something else"); ok {
		t.Error("a body books does not name was converted")
	}
}

func TestEdit_KeepsWhatIsBlank(t *testing.T) {
	stored := org.Bot{Name: "Dana", Role: "Analyst", Tier: org.TierFast, ModelOverride: "anthropic::m"}
	got, err := edit(stored, &hearthv1.UpdateBotRequest{Role: " ", Avatar: "star"})
	if err != nil || got.Name != "Dana" || got.Role != "Analyst" || got.Tier != org.TierFast || got.Avatar != "star" || got.ModelOverride != "anthropic::m" {
		t.Errorf("edit = %+v, %v", got, err)
	}
	if _, err := edit(stored, &hearthv1.UpdateBotRequest{ModelOverride: "m"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("edit with a bare model = %v, want invalid_argument", err)
	}
	if got, _ := edit(stored, &hearthv1.UpdateBotRequest{ClearModelOverride: true}); got.ModelOverride != "" {
		t.Errorf("clear_model_override kept %q", got.ModelOverride)
	}
}
