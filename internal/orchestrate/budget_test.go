package orchestrate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rfizzle/llm"
	"github.com/rfizzle/llm/pricing"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/usage"
)

// spendThenHang reports a request of n prompt tokens and then waits for the
// request to end.
func spendThenHang(n int) step {
	return func(ctx context.Context) <-chan llm.StreamEvent {
		ch := make(chan llm.StreamEvent)
		go func() {
			defer close(ch)
			select {
			case ch <- llm.StreamEvent{Usage: &llm.Usage{PromptTokens: n}}:
			case <-ctx.Done():
				return
			}
			<-ctx.Done()
		}()
		return ch
	}
}

// metered is a fixture whose service meters at $1 per million prompt tokens
// and stops the run through StopForBudget.
func metered(t *testing.T, steps ...step) (*fixture, *meter.Meter) {
	t.Helper()
	f := newFixture(t, steps...)
	table := pricing.NewTable(map[string]pricing.ModelPricing{"known": {InputCostPerToken: 1e-6}})
	var svc *Service
	m := meter.New(f.db, table, nil, nil, func(runID, reason string) { svc.StopForBudget(runID, reason) })
	d := f.svc.d
	d.Meter = m
	d.Models = func(org.Bot) agent.Model { return agent.Model{Provider: "openai", Name: "known"} }
	var err error
	if svc, err = Open(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	f.svc = svc
	return f, m
}

func TestBudget_HardLimitStopsTheRunAndRefusesTheNextMessage(t *testing.T) {
	f, m := metered(t, spendThenHang(2_000_000)) // $2 against a $1 budget
	ctx := context.Background()
	if err := m.SetBudgets(ctx, []usage.Budget{{Subject: f.bot.ID, Cents: 100}}); err != nil {
		t.Fatal(err)
	}
	r, _ := f.send(t, "go", ended)
	got := f.storedRun(t, r.ID)
	if got.State != org.RunFailed || got.Failure != meter.SpentReason {
		t.Fatalf("run = %v %q, want failed with %q", got.State, got.Failure, meter.SpentReason)
	}

	_, _, err := f.svc.Send(ctx, f.bot.DMConversationID, "again", nil)
	if !errors.Is(err, ErrPaused) || !strings.Contains(err.Error(), "Dana is paused") {
		t.Fatalf("Send while paused = %v, want ErrPaused naming Dana", err)
	}
	if _, err := f.svc.Retry(ctx, f.bot.DMConversationID); !errors.Is(err, ErrPaused) {
		t.Fatalf("Retry while paused = %v, want ErrPaused", err)
	}

	if err := m.Resume(ctx, f.bot.ID); err != nil {
		t.Fatal(err)
	}
	f.script.steps = append(f.script.steps, events(llm.StreamEvent{Token: "ok", Done: true}))
	if _, _, err := f.svc.Send(ctx, f.bot.DMConversationID, "again", nil); err != nil {
		t.Fatalf("Send after Resume: %v", err)
	}
}
