package books

import (
	"context"
	"slices"

	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/provider"
	hearthv1 "github.com/rfizzle/hearth/internal/rpc/gen/hearth/v1"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/store"
	storebooks "github.com/rfizzle/hearth/internal/store/books"
)

// Pages is the handbook the HandbookService serves; handbook.Service is one.
type Pages interface {
	List(ctx context.Context, botID string) ([]memory.Page, handbook.Meter, error)
	Put(ctx context.Context, p memory.Page) (memory.Page, error)
	Delete(ctx context.Context, id string) error
}

// Ledger is the meter the UsageService reads and sets; meter.Meter is one.
type Ledger interface {
	Summary(ctx context.Context, month string) (meter.Summary, error)
	SetBudgets(ctx context.Context, list []usage.Budget) error
	Resume(ctx context.Context, subject string) error
}

// Deps is what the services here are handed. Publish tells the bus about a
// change and is never nil.
type Deps struct {
	Store    *store.DB
	Accounts *provider.Accounts
	Pages    Pages
	Meter    Ledger
	Publish  func(body any)
}

// The services, one per file beside this one. Bots and Models are the
// BotService and ProviderService calls rpc's own services take on.
type (
	Bots     struct{ Deps } // BotService: a bot's settings and its notes
	Handbook struct{ Deps } // HandbookService: the pages every bot reads
	Usage    struct{ Deps } // UsageService: the month's cost and the budgets
	Models   struct{ Deps } // ProviderService: the tiers and the models on offer
)

// PageChanged is a handbook page saved, or removed when Deleted is set.
type PageChanged struct {
	Page    memory.Page
	Deleted bool
}

// TiersChanged is the tier mapping after the owner changed it.
type TiersChanged struct{ Tiers []storebooks.TierChoice }

// UsageTicked is a bot's month after one of its requests, with its budget,
// and the org's month.
type UsageTicked struct {
	Month string
	Bot   meter.BotSummary
	Org   usage.Totals
}

// Ticks is the function the meter hears each request's totals on: it
// publishes them with the bot's budget and pause from the month's summary,
// so the row the app replaces keeps them. A summary that cannot be read
// leaves the budget out rather than the tick.
func Ticks(l Ledger, publish func(body any)) func(meter.Tick) {
	return func(t meter.Tick) {
		bot := meter.BotSummary{BotTotals: usage.BotTotals{BotID: t.BotID, Totals: t.Bot}}
		if sum, err := l.Summary(context.Background(), t.Month); err == nil {
			if i := slices.IndexFunc(sum.Bots, func(b meter.BotSummary) bool { return b.BotID == t.BotID }); i >= 0 {
				bot.Budget, bot.Paused = sum.Bots[i].Budget, sum.Bots[i].Paused
			}
		}
		publish(UsageTicked{Month: t.Month, Bot: bot, Org: t.Org})
	}
}

// Event is the SubscribeResponse, without its sequence, for a body this
// package converts, and false for any other.
func Event(body any) (*hearthv1.SubscribeResponse, bool) {
	out := &hearthv1.SubscribeResponse{}
	switch b := body.(type) {
	case wire.NoteChanged:
		changed := &hearthv1.NoteChanged{Note: noteToProto(b.Note), Deleted: b.Deleted, BotId: b.BotID}
		out.Event = &hearthv1.SubscribeResponse_NoteChanged{NoteChanged: changed}
	case PageChanged:
		changed := &hearthv1.HandbookChanged{Page: pageToProto(b.Page), Deleted: b.Deleted}
		out.Event = &hearthv1.SubscribeResponse_HandbookChanged{HandbookChanged: changed}
	case TiersChanged:
		out.Event = &hearthv1.SubscribeResponse_TiersChanged{TiersChanged: &hearthv1.TiersChanged{Tiers: wire.Each(b.Tiers, tierToProto)}}
	case UsageTicked:
		tick := &hearthv1.UsageTick{Bot: botUsageToProto(b.Bot), Org: totalsToProto(b.Org), Month: b.Month}
		out.Event = &hearthv1.SubscribeResponse_UsageTick{UsageTick: tick}
	default:
		return nil, false
	}
	return out, true
}
