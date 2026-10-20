package meter

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rfizzle/llm/pricing"

	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/store"
)

// PlanProvider is the provider whose requests the owner's plan pays for: the
// tokens are counted and nothing is priced. It is agentcore's rule, copied
// here because hearth keeps its own ledger and agentcore's lives in memory.
const PlanProvider = "chatgpt"

// SpentReason is the failure a run interrupted by the hard limit ends with.
const SpentReason = "this month's budget is spent"

// Tick is the month's totals after a request: the bot's and the org's.
type Tick struct {
	BotID string
	Month string
	Bot   usage.Totals
	Org   usage.Totals
}

// BotSummary is one bot's month with its budget.
type BotSummary struct {
	usage.BotTotals
	Budget usage.Budget
	Paused bool
}

// Summary is a month's spending: the org's, each bot's, and the months there
// is usage for, the latest first.
type Summary struct {
	Month     string
	Months    []string
	Org       usage.Totals
	OrgBudget usage.Budget
	OrgPaused bool
	Bots      []BotSummary
}

// Meter prices and records requests and enforces the budgets.
type Meter struct {
	db      *store.DB
	table   *pricing.Table
	onTick  func(Tick)
	onInbox func(inbox.Item)
	onPause func(runID, reason string)
	now     func() time.Time

	mu   sync.Mutex
	soft map[string]bool // subject|month|cents the owner was told of
}

// New is a meter over db pricing with table, which may be nil to price
// nothing. OnTick hears the totals after every request, OnInbox each item
// the meter opens or closes, and OnPause the run to stop when the request it
// made finds its bot or the org paused. Each may be nil.
func New(db *store.DB, table *pricing.Table, onTick func(Tick), onInbox func(inbox.Item), onPause func(runID, reason string)) *Meter {
	return &Meter{db: db, table: table, onTick: onTick, onInbox: onInbox, onPause: onPause,
		now: time.Now, soft: map[string]bool{}}
}

// Record prices r, stores it, tells OnTick the month's totals and checks the
// budgets of r's bot and of the org. r.At is when the request was made; zero
// is now.
func (m *Meter) Record(ctx context.Context, r usage.Record) error {
	if r.At.IsZero() {
		r.At = m.now()
	}
	r.Plan, r.Priced, r.CostMicros = false, false, 0
	switch {
	case r.Provider == PlanProvider:
		r.Plan = true
	case m.table != nil:
		if cost, found := m.table.Price(r.Model, r.Prompt, r.Completion, r.Cached, r.Created); found {
			r.Priced = true
			r.CostMicros = int64(math.Round(cost * 1e6))
		}
	}
	if err := m.db.AddUsage(ctx, r); err != nil {
		return err
	}
	month := usage.Month(r.At)
	bots, all, err := m.db.UsageByMonth(ctx, month)
	if err != nil {
		return err
	}
	var mine usage.Totals
	for _, b := range bots {
		if b.BotID == r.BotID {
			mine = b.Totals
		}
	}
	if m.onTick != nil {
		m.onTick(Tick{BotID: r.BotID, Month: month, Bot: mine, Org: all})
	}
	return m.check(ctx, r, month, mine, all)
}

// check applies the rails to the bot's budget and the org's.
func (m *Meter) check(ctx context.Context, r usage.Record, month string, bot, all usage.Totals) error {
	budgets, err := m.budgets(ctx)
	if err != nil {
		return err
	}
	for _, s := range []struct {
		subject string
		totals  usage.Totals
	}{{r.BotID, bot}, {usage.Org, all}} {
		b, ok := budgets[s.subject]
		if !ok {
			continue
		}
		_, soft, hard := usage.Level(s.totals, b)
		switch {
		case b.PausedMonth == month:
			// Already paused: a run still going is stopped again.
			m.stop(r.RunID)
		case hard && b.ResumedMonth != month:
			if err := m.pause(ctx, r, b); err != nil {
				return err
			}
		case soft:
			if err := m.warn(ctx, r, b, month); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Meter) stop(runID string) {
	if m.onPause != nil && runID != "" {
		m.onPause(runID, SpentReason)
	}
}

func (m *Meter) pause(ctx context.Context, r usage.Record, b usage.Budget) error {
	if err := m.db.SetPaused(ctx, b.Subject, usage.Month(r.At), ""); err != nil {
		return err
	}
	who := m.who(ctx, b.Subject)
	if err := m.open(ctx, inbox.Item{
		Kind: inbox.Budget, BotID: r.BotID,
		Title:  fmt.Sprintf("%s paused: this month's %s is spent", who, dollars(b.Cents)),
		Effect: fmt.Sprintf("Paused: %s used all of this month's %s. Resume to carry on this month, or raise the budget.", who, dollars(b.Cents)),
	}); err != nil {
		return err
	}
	m.stop(r.RunID)
	return nil
}

func (m *Meter) warn(ctx context.Context, r usage.Record, b usage.Budget, month string) error {
	key := fmt.Sprintf("%s|%s|%d", b.Subject, month, b.Cents)
	title := fmt.Sprintf("%s has used 80%% of this month's %s", m.who(ctx, b.Subject), dollars(b.Cents))
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.soft[key] {
		return nil
	}
	m.soft[key] = true
	items, err := m.db.InboxItems(ctx, false)
	if err != nil {
		return err
	}
	for _, it := range items {
		if it.Kind == inbox.Budget && it.Title == title && usage.Month(it.CreatedAt) == month {
			return nil
		}
	}
	return m.open(ctx, inbox.Item{Kind: inbox.Budget, BotID: r.BotID, Title: title,
		Effect: title + ". Work pauses at the whole amount."})
}

// open stores it, which is not tied to a conversation so it never holds one,
// and tells OnInbox.
func (m *Meter) open(ctx context.Context, it inbox.Item) error {
	stored, err := m.db.CreateInboxItem(ctx, it)
	if err != nil {
		return err
	}
	if m.onInbox != nil {
		m.onInbox(stored)
	}
	return nil
}

// who is how items name subject: the bot's name, or the org.
func (m *Meter) who(ctx context.Context, subject string) string {
	if subject == usage.Org {
		return "The org"
	}
	if b, err := m.db.Bot(ctx, subject); err == nil {
		return b.Name
	}
	return "A bot"
}

// dollars is cents as the owner reads a limit: $20, or $20.50.
func dollars(cents int64) string {
	if cents%100 == 0 {
		return fmt.Sprintf("$%d", cents/100)
	}
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}

func (m *Meter) budgets(ctx context.Context) (map[string]usage.Budget, error) {
	list, err := m.db.Budgets(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]usage.Budget, len(list))
	for _, b := range list {
		out[b.Subject] = b
	}
	return out, nil
}

// Paused is whether botID may not start work this month: its own budget or
// the org's is spent and not resumed.
func (m *Meter) Paused(ctx context.Context, botID string) (bool, error) {
	budgets, err := m.budgets(ctx)
	if err != nil {
		return false, err
	}
	month := usage.Month(m.now())
	return budgets[botID].PausedMonth == month || budgets[usage.Org].PausedMonth == month, nil
}

// Summary is month's spending ("2026-10"; empty is this month) with the
// budgets of the org and of every bot that has spent or has one.
func (m *Meter) Summary(ctx context.Context, month string) (Summary, error) {
	now := usage.Month(m.now())
	if month == "" {
		month = now
	}
	bots, all, err := m.db.UsageByMonth(ctx, month)
	if err != nil {
		return Summary{}, err
	}
	months, err := m.db.UsageMonths(ctx)
	if err != nil {
		return Summary{}, err
	}
	budgets, err := m.budgets(ctx)
	if err != nil {
		return Summary{}, err
	}
	s := Summary{Month: month, Months: months, Org: all, OrgBudget: budgets[usage.Org],
		OrgPaused: month == now && budgets[usage.Org].PausedMonth == now}
	seen := map[string]bool{}
	add := func(t usage.BotTotals) {
		seen[t.BotID] = true
		b := budgets[t.BotID]
		s.Bots = append(s.Bots, BotSummary{BotTotals: t, Budget: b, Paused: month == now && b.PausedMonth == now})
	}
	for _, t := range bots {
		add(t)
	}
	for id := range budgets {
		if id != usage.Org && !seen[id] {
			add(usage.BotTotals{BotID: id})
		}
	}
	slices.SortFunc(s.Bots, func(a, b BotSummary) int { return strings.Compare(a.BotID, b.BotID) })
	return s, nil
}

// SetBudgets sets the limit of each budget's subject. A limit raised, or
// removed, lifts the hard limit's exemption a resume gave, and a pause the
// new limit is above the spend of ends.
func (m *Meter) SetBudgets(ctx context.Context, list []usage.Budget) error {
	old, err := m.budgets(ctx)
	if err != nil {
		return err
	}
	month := usage.Month(m.now())
	bots, all, err := m.db.UsageByMonth(ctx, month)
	if err != nil {
		return err
	}
	for _, b := range list {
		if err := m.db.PutBudget(ctx, b); err != nil {
			return err
		}
		cur := old[b.Subject]
		resumed := cur.ResumedMonth
		if b.Cents > cur.Cents || b.Cents <= 0 {
			resumed = ""
		}
		paused := cur.PausedMonth
		if paused == month {
			spent := all
			if b.Subject != usage.Org {
				spent = usage.Totals{}
				for _, t := range bots {
					if t.BotID == b.Subject {
						spent = t.Totals
					}
				}
			}
			if _, _, hard := usage.Level(spent, b); !hard {
				paused = ""
			}
		}
		if paused != cur.PausedMonth || resumed != cur.ResumedMonth {
			if err := m.db.SetPaused(ctx, b.Subject, paused, resumed); err != nil {
				return err
			}
		}
		if paused == "" && cur.PausedMonth == month {
			if err := m.closePauseItems(ctx, b.Subject, "Budget raised"); err != nil {
				return err
			}
		}
	}
	return nil
}

// Resume lets subject (a bot id, or usage.Org) work again this month: the
// pause ends and the hard limit is off until the month changes or the budget
// is raised.
func (m *Meter) Resume(ctx context.Context, subject string) error {
	if err := m.db.SetPaused(ctx, subject, "", usage.Month(m.now())); err != nil {
		return err
	}
	return m.closePauseItems(ctx, subject, "Resumed")
}

// closePauseItems closes the open pause items of subject.
func (m *Meter) closePauseItems(ctx context.Context, subject, outcome string) error {
	items, err := m.db.InboxItems(ctx, true)
	if err != nil {
		return err
	}
	who := m.who(ctx, subject)
	for _, it := range items {
		if it.Kind != inbox.Budget || !strings.HasPrefix(it.Effect, "Paused: ") || !strings.HasPrefix(it.Title, who+" paused") {
			continue
		}
		if err := m.db.CloseInboxItem(ctx, it.ID, 0, "", outcome); err != nil {
			return err
		}
		if m.onInbox != nil {
			if closed, err := m.db.InboxItem(ctx, it.ID); err == nil {
				m.onInbox(closed)
			}
		}
	}
	return nil
}
