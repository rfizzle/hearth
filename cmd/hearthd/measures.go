package main

import (
	"context"
	"path/filepath"

	"github.com/rfizzle/hearth/internal/handbook"
	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/orchestrate"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/rpc/deliverables"
	"github.com/rfizzle/hearth/internal/store"
)

// measures wires the sources of the measure registry: the store's bots and
// projects, the project rule through the orchestrator, a month of the meter's
// usage, the Inbox's open items, a bot's notes and the handbook's meter.
func measures(db *store.DB, svc *orchestrate.Service, m *meter.Meter, pages *handbook.Service) measure.Deps {
	return measure.Deps{
		Roster: db, Projects: db, Status: svc.Status,
		Ledger: monthOf{m}, Inbox: openItems{db}, Notes: noteCount{db}, Handbook: handbookFill{pages},
	}
}

// deliverableDeps adds where the Hearth folder is, read on each call since
// the owner may name or move it while the core runs.
func deliverableDeps(db *store.DB, d measure.Deps) deliverables.Deps {
	return deliverables.Deps{Measures: d, Root: func() string {
		o, found, err := db.FindOrg(context.Background())
		if err != nil || !found {
			return ""
		}
		return o.WorkspacePath
	}}
}

// templatesFolders are the owner's templates folder as given and as it
// resolves, for the tools and the grants to keep away from.
func templatesFolders(hearth string) []string {
	return reservedFolders(filepath.Join(hearth, "Templates"))
}

type monthOf struct{ m *meter.Meter }

func (l monthOf) Month(ctx context.Context, month string) (measure.Month, error) {
	s, err := l.m.Summary(ctx, month)
	if err != nil {
		return measure.Month{}, err
	}
	out := measure.Month{Org: spend(s.Org, s.OrgBudget), Bots: map[string]measure.Spend{}}
	for _, b := range s.Bots {
		out.Bots[b.BotID] = spend(b.Totals, b.Budget)
	}
	return out, nil
}

func spend(t usage.Totals, b usage.Budget) measure.Spend {
	return measure.Spend{CostMicros: t.CostMicros, Tokens: t.Prompt + t.Completion, BudgetCents: b.Cents, Priced: t.Unpriced == 0}
}

type openItems struct{ db *store.DB }

func (o openItems) Open(ctx context.Context) (int, error) {
	items, err := o.db.InboxItems(ctx, true)
	return len(items), err
}

type noteCount struct{ db *store.DB }

func (n noteCount) Count(ctx context.Context, botID string) (int, error) {
	notes, err := n.db.Notes(ctx, botID)
	return len(notes), err
}

type handbookFill struct{ pages *handbook.Service }

func (h handbookFill) Used(ctx context.Context, botID string) (int64, int64, error) {
	_, fill, err := h.pages.List(ctx, botID)
	return fill.Used, fill.Cap, err
}
