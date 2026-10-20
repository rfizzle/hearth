package books

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// AddUsage stores one model request. Its month is the month of r.At, which
// is now when it is zero.
func (repo *Repo) AddUsage(ctx context.Context, r usage.Record) error {
	if r.At.IsZero() {
		r.At = repo.clock()
	}
	if _, err := repo.sql.ExecContext(ctx, `INSERT INTO usage
		(run_id, bot_id, conversation_id, month, provider, model, prompt, completion, cached, created, plan,
		cost_micros, priced, at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RunID, r.BotID, r.ConversationID, usage.Month(r.At), r.Provider, r.Model, r.Prompt, r.Completion,
		r.Cached, r.Created, r.Plan, r.CostMicros, r.Priced, r.At.UnixMilli()); err != nil {
		return fmt.Errorf("add usage: %w", err)
	}
	return nil
}

// totalsColumns sums a group of usage rows into usage.Totals.
const totalsColumns = `COUNT(*), COALESCE(SUM(prompt), 0), COALESCE(SUM(completion), 0), COALESCE(SUM(cached), 0),
	COALESCE(SUM(created), 0), COALESCE(SUM(cost_micros), 0), COALESCE(SUM(plan = 0 AND priced = 0), 0)`

// UsageByMonth is what each bot spent in month ("2026-10"), by bot id, and
// the total over all of them. A month with no requests has neither.
func (repo *Repo) UsageByMonth(ctx context.Context, month string) ([]usage.BotTotals, usage.Totals, error) {
	scan := func(row sqlrow.Scanner) (usage.BotTotals, error) {
		var b usage.BotTotals
		err := row.Scan(&b.BotID, &b.Requests, &b.Prompt, &b.Completion, &b.Cached, &b.Created, &b.CostMicros, &b.Unpriced)
		return b, err
	}
	bots, err := sqlrow.QueryAll(ctx, repo.sql, "usage", scan,
		`SELECT bot_id, `+totalsColumns+` FROM usage WHERE month = ? GROUP BY bot_id ORDER BY bot_id`, month)
	if err != nil {
		return nil, usage.Totals{}, err
	}
	var all usage.Totals
	for _, b := range bots {
		all.Add(b.Totals)
	}
	return bots, all, nil
}

// UsageMonths is every month with usage, the latest first.
func (repo *Repo) UsageMonths(ctx context.Context) ([]string, error) {
	return sqlrow.QueryAll(ctx, repo.sql, "usage months", func(row sqlrow.Scanner) (string, error) {
		var m string
		return m, row.Scan(&m)
	}, `SELECT DISTINCT month FROM usage ORDER BY month DESC`)
}
