package meter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rfizzle/llm/pricing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/store"
)

// fixture is a meter over a temp-dir store with one hired bot and a price
// table with one model, at $1 per million input tokens and $2 per million
// output tokens, and a record of what the seams were told.
type fixture struct {
	t      *testing.T
	db     *store.DB
	m      *Meter
	bot    org.Bot
	ticks  []Tick
	items  []inbox.Item
	pauses []string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	root := filepath.Join(dir, "Hearth")
	if err := db.PutOrg(ctx, org.Org{Name: "Acme", WorkspacePath: root}); err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Finds things out.", org.TierBalanced, root)
	if bot, err = db.CreateBot(ctx, bot, grants, dm); err != nil {
		t.Fatal(err)
	}
	table := pricing.NewTable(map[string]pricing.ModelPricing{
		"known": {InputCostPerToken: 1e-6, OutputCostPerToken: 2e-6},
	})
	f := &fixture{t: t, db: db, bot: bot}
	f.m = New(db, table,
		func(tk Tick) { f.ticks = append(f.ticks, tk) },
		func(it inbox.Item) { f.items = append(f.items, it) },
		func(runID, reason string) { f.pauses = append(f.pauses, runID+": "+reason) })
	return f
}

// request records one request of the bot's on the model.
func (f *fixture) request(provider, model string, prompt, completion int64) {
	f.t.Helper()
	err := f.m.Record(context.Background(), usage.Record{
		RunID: "run1", BotID: f.bot.ID, Provider: provider, Model: model, Prompt: prompt, Completion: completion,
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) month() usage.Totals {
	f.t.Helper()
	_, all, err := f.db.UsageByMonth(context.Background(), usage.Month(f.m.now()))
	if err != nil {
		f.t.Fatal(err)
	}
	return all
}

func (f *fixture) budget(subject string, cents int64) {
	f.t.Helper()
	if err := f.m.SetBudgets(context.Background(), []usage.Budget{{Subject: subject, Cents: cents}}); err != nil {
		f.t.Fatal(err)
	}
}

func TestRecordPricesRequests(t *testing.T) {
	f := newFixture(t)
	f.request("openai", "known", 1_000_000, 0)     // $1.00
	f.request("openai", "known", 0, 500_000)       // $1.00
	f.request("openai", "known", 250_000, 250_000) // $0.75
	tot := f.month()
	if tot.Requests != 3 || tot.CostMicros != 2_750_000 || tot.Unpriced != 0 {
		t.Fatalf("totals = %+v, want 3 requests, 2750000 micro-dollars, none unpriced", tot)
	}
}

func TestPlanTokensAreCountedNotPriced(t *testing.T) {
	f := newFixture(t)
	f.request(PlanProvider, "known", 1_000_000, 1_000_000)
	tot := f.month()
	if tot.Prompt != 1_000_000 || tot.Completion != 1_000_000 || tot.CostMicros != 0 || tot.Unpriced != 0 {
		t.Fatalf("totals = %+v, want tokens counted at no cost and not unpriced", tot)
	}
}

func TestUnknownModelIsUnpriced(t *testing.T) {
	f := newFixture(t)
	f.request("openai", "mystery", 1_000_000, 1_000_000)
	tot := f.month()
	if tot.Unpriced != 1 || tot.CostMicros != 0 || tot.Prompt != 1_000_000 {
		t.Fatalf("totals = %+v, want one unpriced request with its tokens counted", tot)
	}
}

func TestTickPerRecord(t *testing.T) {
	f := newFixture(t)
	f.request("openai", "known", 1_000_000, 0)
	f.request("openai", "known", 1_000_000, 0)
	if len(f.ticks) != 2 {
		t.Fatalf("ticks = %d, want one per record", len(f.ticks))
	}
	last := f.ticks[1]
	if last.BotID != f.bot.ID || last.Bot.CostMicros != 2_000_000 || last.Org.CostMicros != 2_000_000 || last.Bot.Requests != 2 {
		t.Fatalf("last tick = %+v, want the month's totals of the bot and the org", last)
	}
}

func TestSoftLimitOpensOneItem(t *testing.T) {
	f := newFixture(t)
	f.budget(f.bot.ID, 1000) // $10
	f.request("openai", "known", 7_000_000, 0)
	if len(f.items) != 0 {
		t.Fatalf("items at 70%% = %d, want none", len(f.items))
	}
	f.request("openai", "known", 1_500_000, 0) // 85%
	f.request("openai", "known", 100_000, 0)   // 86%
	if len(f.items) != 1 || f.items[0].Kind != inbox.Budget {
		t.Fatalf("items = %+v, want exactly one budget item", f.items)
	}
	if got, want := f.items[0].Title, "Dana has used 80% of this month's $10"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if len(f.pauses) != 0 {
		t.Fatalf("pauses = %v, want none below the hard line", f.pauses)
	}
	// A meter started afresh (a relaunch) does not tell the owner again.
	again := New(f.db, nil, nil, func(inbox.Item) { t.Error("second soft item after relaunch") }, nil)
	if err := again.Record(context.Background(), usage.Record{RunID: "run1", BotID: f.bot.ID, Provider: "x", Model: "y"}); err != nil {
		t.Fatal(err)
	}
}

func TestHardLimitPausesAndResumeLetsWork(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.budget(f.bot.ID, 1000)
	f.request("openai", "known", 10_000_000, 0) // 100%
	if len(f.pauses) != 1 || f.pauses[0] != "run1: "+SpentReason {
		t.Fatalf("pauses = %v, want the run stopped with the reason", f.pauses)
	}
	if paused, err := f.m.Paused(ctx, f.bot.ID); err != nil || !paused {
		t.Fatalf("Paused = %v, %v; want true", paused, err)
	}
	if len(f.items) != 1 || f.items[0].Kind != inbox.Budget || f.items[0].State != inbox.Open {
		t.Fatalf("items = %+v, want one open budget item", f.items)
	}
	if f.items[0].ConversationID != "" {
		t.Fatalf("the pause item holds conversation %q; it must hold none", f.items[0].ConversationID)
	}

	if err := f.m.Resume(ctx, f.bot.ID); err != nil {
		t.Fatal(err)
	}
	if paused, _ := f.m.Paused(ctx, f.bot.ID); paused {
		t.Fatal("still paused after Resume")
	}
	if last := f.items[len(f.items)-1]; last.State != inbox.Closed {
		t.Fatalf("the pause item is %v after Resume, want closed", last.State)
	}
	// Resumed: more spending this month does not pause again.
	f.request("openai", "known", 1_000_000, 0)
	if paused, _ := f.m.Paused(ctx, f.bot.ID); paused {
		t.Fatal("paused again in the month it was resumed")
	}
	// Until the budget is raised and spent through.
	f.budget(f.bot.ID, 1200) // $12; spend is $11
	f.request("openai", "known", 1_000_000, 0)
	if paused, _ := f.m.Paused(ctx, f.bot.ID); !paused {
		t.Fatal("not paused after a raised budget was spent")
	}
}

func TestRaisedBudgetClearsPause(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.budget(f.bot.ID, 1000)
	f.request("openai", "known", 10_000_000, 0)
	f.budget(f.bot.ID, 500) // lowered: still paused
	if paused, _ := f.m.Paused(ctx, f.bot.ID); !paused {
		t.Fatal("a lowered budget ended the pause")
	}
	f.budget(f.bot.ID, 5000)
	if paused, _ := f.m.Paused(ctx, f.bot.ID); paused {
		t.Fatal("still paused with a budget above the spend")
	}
}

func TestOrgBudgetPausesEveryBotAndStopsRunsStillGoing(t *testing.T) {
	f := newFixture(t)
	f.budget(usage.Org, 100) // $1
	f.request("openai", "known", 1_000_000, 0)
	if paused, _ := f.m.Paused(context.Background(), "any-bot"); !paused {
		t.Fatal("the org is spent but another bot is not paused")
	}
	f.request(PlanProvider, "known", 10, 10) // a run still going
	if len(f.pauses) != 2 {
		t.Fatalf("pauses = %v, want the run stopped again on its next request", f.pauses)
	}
	s, err := f.m.Summary(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !s.OrgPaused || s.OrgBudget.Cents != 100 || len(s.Bots) != 1 || s.Org.CostMicros != 1_000_000 {
		t.Fatalf("summary = %+v", s)
	}
}
