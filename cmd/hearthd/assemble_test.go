package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/rfizzle/llm"

	"github.com/rfizzle/hearth/internal/agent"
	"github.com/rfizzle/hearth/internal/measure"
	"github.com/rfizzle/hearth/internal/meter"
	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/rpc"
	"github.com/rfizzle/hearth/internal/rpc/books"
	"github.com/rfizzle/hearth/internal/rpc/wire"
	"github.com/rfizzle/hearth/internal/store"
)

func TestDrafter_ANoModelIsUnavailableAndAReplyIsAProposal(t *testing.T) {
	none := func(org.Bot) (agent.Stream, error) { return nil, errors.New("no account is signed in") }
	_, err := drafter(nil, none, nil)(t.Context(), rpc.DraftRequest{Description: "help", Tier: org.TierFast})
	if !errors.Is(err, wire.ErrUnavailable) || !strings.Contains(err.Error(), "no account is signed in") {
		t.Errorf("err = %v, want unavailable in the owner's words", err)
	}

	say := func(org.Bot) (agent.Stream, error) {
		return func(context.Context, []llm.Message, []llm.Tool, string) (<-chan llm.StreamEvent, error) {
			ch := make(chan llm.StreamEvent, 2)
			ch <- llm.StreamEvent{Token: `{"name":"Wren","role":"Analyst","job_description":"Read.","tier":"best"}`}
			ch <- llm.StreamEvent{Done: true}
			close(ch)
			return ch, nil
		}, nil
	}
	got, err := drafter(nil, say, nil)(t.Context(), rpc.DraftRequest{Description: "help", Tier: org.TierFast})
	if err != nil || got.Name != "Wren" || got.Tier != org.TierBest {
		t.Errorf("draft = %+v, %v", got, err)
	}
}

func TestOpenCore_GivesOlderBotsTheRightToAsk(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Reads.", org.TierFast, filepath.Join(dir, "Hearth"))
	var older []org.Grant
	for _, g := range grants {
		if g.Capability != org.AskOwner {
			older = append(older, g)
		}
	}
	bot, err = db.CreateBot(t.Context(), bot, older, dm)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, dir)
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	got, err := opts.Store.GrantsFor(ctx, bot.ID)
	if err != nil || org.Allowed(got, org.AskOwner) != org.ModeAllowed {
		t.Errorf("grants after open = %v, %v, want the right to ask", got, err)
	}
}

func TestOpenCore_GivesAnOlderOrgItsChiefOfStaffOnce(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "Hearth")
	if err := db.PutOrg(t.Context(), org.Org{Name: "Acme", WorkspacePath: root}); err != nil {
		t.Fatal(err)
	}
	bot, grants, dm := org.NewBot("Dana", "Analyst", "Reads.", org.TierFast, root)
	if _, err := db.CreateBot(t.Context(), bot, grants, dm); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		ctx, cancel := context.WithCancel(context.Background())
		opts, closeCore, err := openCore(ctx, dir)
		if err != nil {
			cancel()
			t.Fatalf("openCore: %v", err)
		}
		cos, err := opts.Store.Coordinator(ctx)
		bots, _ := opts.Store.Bots(ctx)
		closeCore()
		cancel()
		if err != nil || cos.Name != "Chief of Staff" || cos.Scope != org.ScopeOrg || cos.DMConversationID == "" {
			t.Fatalf("Coordinator after open = %+v, %v", cos, err)
		}
		if len(bots) != 2 {
			t.Fatalf("bots after open = %+v, want Dana and one Chief of Staff", bots)
		}
	}
}

func TestOpenCore_HiresNobodyBeforeOnboarding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if bots, err := opts.Store.Bots(ctx); err != nil || len(bots) != 0 {
		t.Fatalf("bots before onboarding = %+v, %v, want none", bots, err)
	}
}

func TestOpenCore_TheDataFolderIsReservedForGrantsAndTools(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, dir)
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if len(opts.ReservedFolders) == 0 || opts.ReservedFolders[0] != dir {
		t.Errorf("ReservedFolders = %v, want the data folder", opts.ReservedFolders)
	}
	if got := reservedFolders(filepath.Join(dir, "x", "..")); got[0] != dir {
		t.Errorf("reservedFolders = %v, want the cleaned path", got)
	}
}

func TestOpenCore_TheTemplatesFolderIsReservedAndTheMeasuresAreWired(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "Hearth")
	db, err := store.Open(filepath.Join(dir, "hearth.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.PutOrg(t.Context(), org.Org{Name: "Acme", WorkspacePath: root}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, dir)
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if !slices.Contains(opts.ReservedFolders, filepath.Join(root, "Templates")) {
		t.Errorf("ReservedFolders = %v, want the Templates folder of the Hearth folder", opts.ReservedFolders)
	}
	if got := opts.Deliverables.Root(); got != root {
		t.Errorf("Root = %q, want %q", got, root)
	}
	ids := []measure.ID{{Family: "team.size"}, {Family: "inbox.open"}, {Family: "usage.cost", Subject: "org", Period: "2026-10"}}
	vals, err := opts.Deliverables.Measures.Read(ctx, ids)
	if err != nil || len(vals) != 3 || vals[0].Missing != "" || vals[2].Missing != "" {
		t.Errorf("Read = %+v, %v, want the core's own figures", vals, err)
	}
}

func TestOpenCore_TheOrgFolderIsMadeByTheWorkspacePackage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if opts.EnsureWorkspace == nil {
		t.Fatal("EnsureWorkspace is not set: UpdateOrg would refuse every folder")
	}
	root := filepath.Join(t.TempDir(), "Hearth")
	if err := opts.EnsureWorkspace(root); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(root, "Inbox")); err != nil || !fi.IsDir() {
		t.Fatalf("the Hearth folder was not made: %v", err)
	}
}

func TestOpenCore_AMeteredRequestTicksOnTheBusWithTheBudget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	opts, closeCore, err := openCore(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("openCore: %v", err)
	}
	t.Cleanup(closeCore)
	if opts.Handbook == nil {
		t.Error("the handbook is not set: HandbookService would fail every call")
	}
	if err := opts.Meter.SetBudgets(ctx, []usage.Budget{{Subject: "b1", Cents: 500}}); err != nil {
		t.Fatal(err)
	}
	events, stop := opts.Bus.Subscribe(0)
	defer stop()
	if err := opts.Meter.(*meter.Meter).Record(ctx, usage.Record{BotID: "b1", Provider: "anthropic", Model: "unknown", Prompt: 10}); err != nil {
		t.Fatal(err)
	}
	guard := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			tick, ok := e.Body.(books.UsageTicked)
			if !ok {
				continue
			}
			if tick.Bot.BotID != "b1" || tick.Bot.Prompt != 10 || tick.Bot.Budget.Cents != 500 {
				t.Errorf("tick = %+v, want b1's 10 tokens and its budget", tick)
			}
			return
		case <-guard:
			t.Fatal("no usage tick reached the bus")
		}
	}
}
