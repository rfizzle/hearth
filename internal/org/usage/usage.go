// Package usage holds what the bots have spent: one record per model
// request, the totals over a month, the budgets the owner sets and the pure
// rule that says how near a budget the spending is.
//
// Responsibility: plain types and pure rules. The store keeps the rows and
// the meter prices requests and acts on the levels.
//
// Invariants the types do not hold:
//   - The package imports org (when it needs to) and the standard library
//     only, and does no I/O.
//   - Money is whole micro-dollars (CostMicros) and budgets whole cents.
//   - A request that is not priced, or that a plan covers, costs 0 here;
//     its tokens are still counted.
//
// See docs/model.md.
package usage

import "time"

// Record is one model request: whose it was, which model served it, the
// tokens it used and what it cost. Plan is true when the owner's plan paid
// for it, Priced when the price table knew the model.
type Record struct {
	RunID          string
	BotID          string
	ConversationID string
	Provider       string
	Model          string
	Prompt         int64
	Completion     int64
	Cached         int64
	Created        int64
	Plan           bool
	Priced         bool
	CostMicros     int64
	At             time.Time
}

// Totals is what a set of records add up to. Unpriced counts the requests
// that were neither priced nor on a plan, whose cost is therefore missing.
type Totals struct {
	Requests   int64
	Prompt     int64
	Completion int64
	Cached     int64
	Created    int64
	CostMicros int64
	Unpriced   int64
}

// Add adds o into t.
func (t *Totals) Add(o Totals) {
	t.Requests += o.Requests
	t.Prompt += o.Prompt
	t.Completion += o.Completion
	t.Cached += o.Cached
	t.Created += o.Created
	t.CostMicros += o.CostMicros
	t.Unpriced += o.Unpriced
}

// BotTotals is the totals for one bot.
type BotTotals struct {
	BotID string
	Totals
}

// Org is the subject of the budget for the whole org; any other subject is a
// bot id.
const Org = "org"

// Budget is a monthly limit in cents for a subject (Org or a bot id), and
// the months the subject was paused for it and resumed in, "" for neither.
type Budget struct {
	Subject      string
	Cents        int64
	PausedMonth  string
	ResumedMonth string
}

// Month is the budget month t falls in, "2026-10", in t's own zone.
func Month(t time.Time) string {
	return t.Format("2006-01")
}

// softAt is the fraction of a budget at which the owner is told.
const softAt = 0.8

// Level is how much of the budget the totals have spent, and whether that
// has reached the soft line (80%) or the hard one (100%). A budget of zero
// or less is no limit: fraction 0 and neither flag. Whether a resumed month
// ignores the hard line is for the caller, which knows the month.
func Level(totals Totals, budget Budget) (fraction float64, soft, hard bool) {
	if budget.Cents <= 0 {
		return 0, false, false
	}
	fraction = float64(totals.CostMicros) / float64(budget.Cents*10_000)
	return fraction, fraction >= softAt, fraction >= 1
}
