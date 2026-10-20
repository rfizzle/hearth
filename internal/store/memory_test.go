package store

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/org/usage"
)

func noteTexts(notes []memory.Note) []string {
	var out []string
	for _, n := range notes {
		out = append(out, n.Text)
	}
	return out
}

func TestNotes_RoundTripKeepingPinsThroughReplace(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	ctx := t.Context()

	a, err := db.AddNote(ctx, memory.Note{BotID: dana.ID, Text: "prefers short answers", ConversationID: "c", MessageID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := db.AddNote(ctx, memory.Note{BotID: dana.ID, Text: "works Tuesdays"})
	if _, err := db.AddNote(ctx, memory.Note{BotID: dana.ID, Text: "likes tea"}); err != nil {
		t.Fatal(err)
	}
	if a.ID == "" || a.CreatedAt.IsZero() {
		t.Fatalf("AddNote did not mint an id and a time: %+v", a)
	}
	if _, err := db.AddNote(ctx, memory.Note{BotID: "nobody", Text: "x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddNote for a missing bot: %v, want ErrNotFound", err)
	}

	if n, err := db.EditNote(ctx, b.ID, "works Wednesdays"); err != nil || n.Text != "works Wednesdays" {
		t.Fatalf("EditNote = %+v, %v", n, err)
	}
	if n, err := db.PinNote(ctx, a.ID, true); err != nil || !n.Pinned {
		t.Fatalf("PinNote = %+v, %v", n, err)
	}
	got, err := db.Notes(ctx, dana.ID)
	if err != nil || !slices.Equal(noteTexts(got), []string{"prefers short answers", "works Wednesdays", "likes tea"}) {
		t.Fatalf("Notes = %v, %v", noteTexts(got), err)
	}
	if got[0].ConversationID != "c" || got[0].MessageID != "m" {
		t.Fatalf("note lost where it was learned: %+v", got[0])
	}

	if err := db.ReplaceNotes(ctx, dana.ID, []memory.Note{{Text: "keeps a Wednesday schedule"}, {Text: "likes tea"}}); err != nil {
		t.Fatal(err)
	}
	after, _ := db.Notes(ctx, dana.ID)
	if !slices.Equal(noteTexts(after), []string{"prefers short answers", "keeps a Wednesday schedule", "likes tea"}) {
		t.Fatalf("after ReplaceNotes = %v", noteTexts(after))
	}
	if after[0].ID != a.ID || !after[0].Pinned {
		t.Fatalf("pinned note was not kept as it was: %+v", after[0])
	}

	if err := db.DeleteNote(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteNote(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeleteNote: %v, want ErrNotFound", err)
	}
	if _, err := db.EditNote(ctx, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("EditNote of a missing note: %v", err)
	}
}

func TestReplaceNotes_FailureLeavesNotesAsTheyWere(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	if _, err := db.AddNote(t.Context(), memory.Note{BotID: dana.ID, Text: "old"}); err != nil {
		t.Fatal(err)
	}
	// A note with a fixed id twice cannot happen through ReplaceNotes, so
	// break the transaction by replacing for a bot that does not exist.
	if err := db.ReplaceNotes(t.Context(), "nobody", []memory.Note{{Text: "new"}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReplaceNotes for a missing bot: %v, want ErrNotFound", err)
	}
	if got, _ := db.Notes(t.Context(), dana.ID); !slices.Equal(noteTexts(got), []string{"old"}) {
		t.Fatalf("notes = %v", noteTexts(got))
	}
}

func TestPages(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	ctx := t.Context()

	all, err := db.PutPage(ctx, memory.Page{Title: "House style", Markdown: "Be brief.", Scope: memory.Scope{Everyone: true}})
	if err != nil {
		t.Fatal(err)
	}
	own, err := db.PutPage(ctx, memory.Page{Title: "Dana only", Markdown: "Dana's rules.", Scope: memory.Scope{BotID: dana.ID}})
	if err != nil {
		t.Fatal(err)
	}
	all.Markdown = "Be very brief."
	if _, err := db.PutPage(ctx, all); err != nil {
		t.Fatal(err)
	}
	pages, err := db.Pages(ctx)
	if err != nil || len(pages) != 2 || pages[0].ID != all.ID || pages[0].Markdown != "Be very brief." ||
		!pages[0].Scope.Everyone || pages[1].Scope.BotID != dana.ID || pages[1].UpdatedAt.IsZero() {
		t.Fatalf("Pages = %+v, %v", pages, err)
	}
	if got := memory.Select(pages, dana.ID); len(got) != 2 {
		t.Fatalf("Dana reads %d pages, want 2", len(got))
	}

	for _, bad := range []memory.Scope{{}, {Everyone: true, BotID: dana.ID}} {
		if _, err := db.PutPage(ctx, memory.Page{Title: "x", Scope: bad}); !errors.Is(err, ErrBadScope) {
			t.Fatalf("PutPage with scope %+v: %v, want ErrBadScope", bad, err)
		}
	}
	if _, err := db.PutPage(ctx, memory.Page{Title: "x", Scope: memory.Scope{BotID: "nobody"}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PutPage for a missing bot: %v, want ErrNotFound", err)
	}
	if err := db.DeletePage(ctx, all.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeletePage(ctx, all.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second DeletePage: %v", err)
	}
	_ = own
}

func TestUsage_SumsPerBotForAMonth(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	oct := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, r := range []usage.Record{
		{BotID: "a", Prompt: 100, Completion: 10, CostMicros: 500, Priced: true, At: oct},
		{BotID: "a", Prompt: 50, Completion: 5, Cached: 20, CostMicros: 250, Priced: true, At: oct.Add(time.Hour)},
		{BotID: "b", Prompt: 7, Plan: true, At: oct},
		{BotID: "b", Prompt: 9, At: oct},
		{BotID: "a", Prompt: 1, CostMicros: 1, Priced: true, At: nov},
	} {
		if err := db.AddUsage(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	bots, total, err := db.UsageByMonth(ctx, "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	wantA := usage.BotTotals{BotID: "a", Totals: usage.Totals{Requests: 2, Prompt: 150, Completion: 15, Cached: 20, CostMicros: 750}}
	wantB := usage.BotTotals{BotID: "b", Totals: usage.Totals{Requests: 2, Prompt: 16, Unpriced: 1}}
	if len(bots) != 2 || bots[0] != wantA || bots[1] != wantB {
		t.Fatalf("by bot = %+v, want %+v and %+v", bots, wantA, wantB)
	}
	if total.Requests != 4 || total.Prompt != 166 || total.CostMicros != 750 || total.Unpriced != 1 {
		t.Fatalf("total = %+v", total)
	}
	if months, err := db.UsageMonths(ctx); err != nil || !slices.Equal(months, []string{"2026-11", "2026-10"}) {
		t.Fatalf("UsageMonths = %v, %v", months, err)
	}
	if bots, total, err := db.UsageByMonth(ctx, "2020-01"); err != nil || len(bots) != 0 || total != (usage.Totals{}) {
		t.Fatalf("an empty month = %+v, %+v, %v", bots, total, err)
	}
}

func TestBudgets_PausedAndResumed(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	if err := db.PutBudget(ctx, usage.Budget{Subject: usage.Org, Cents: 5000}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutBudget(ctx, usage.Budget{Subject: "dana", Cents: 2000}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetPaused(ctx, "dana", "2026-10", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.PutBudget(ctx, usage.Budget{Subject: "dana", Cents: 3000}); err != nil {
		t.Fatal(err)
	}
	got, err := db.Budgets(ctx)
	want := []usage.Budget{{Subject: "dana", Cents: 3000, PausedMonth: "2026-10"}, {Subject: "org", Cents: 5000}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("Budgets = %+v, %v, want %+v", got, err, want)
	}
	if err := db.SetPaused(ctx, "dana", "", "2026-10"); err != nil {
		t.Fatal(err)
	}
	got, _ = db.Budgets(ctx)
	if got[0] != (usage.Budget{Subject: "dana", Cents: 3000, ResumedMonth: "2026-10"}) {
		t.Fatalf("after resume = %+v", got[0])
	}
}

func TestTiers_PutAndRead(t *testing.T) {
	db := newStore(t)
	ctx := t.Context()
	if got, err := db.Tiers(ctx); err != nil || len(got) != 0 {
		t.Fatalf("Tiers on a new store = %+v, %v", got, err)
	}
	first := []TierChoice{
		{Tier: org.TierBest, Provider: "anthropic", Model: "opus"},
		{Tier: org.TierFast, Provider: "openai-compatible", Profile: "local", Model: "llama"},
	}
	if err := db.PutTiers(ctx, first); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Tiers(ctx); err != nil || !slices.Equal(got, []TierChoice{first[1], first[0]}) {
		t.Fatalf("Tiers = %+v, %v", got, err)
	}
	if err := db.PutTiers(ctx, first[:1]); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Tiers(ctx); !slices.Equal(got, first[:1]) {
		t.Fatalf("Tiers after replacing = %+v", got)
	}
}

func TestPutSummary(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	if err := db.PutSummary(t.Context(), dana.DMConversationID, "We agreed on the plan.", 12); err != nil {
		t.Fatal(err)
	}
	c, err := db.Conversation(t.Context(), dana.DMConversationID)
	if err != nil || c.Summary != "We agreed on the plan." || c.SummaryThroughSeq != 12 {
		t.Fatalf("conversation = %+v, %v", c, err)
	}
	if err := db.PutSummary(t.Context(), "nope", "x", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PutSummary on a missing conversation: %v", err)
	}
}

func TestUpdateBot(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	dana.Name, dana.Role, dana.JobDescription = "Dana K", "Editor", "Edits."
	dana.Tier, dana.Avatar, dana.ModelOverride = org.TierBest, "owl", "opus-4"
	dana.FolderPath = "/elsewhere"
	got, err := db.UpdateBot(t.Context(), dana)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Dana K" || got.Role != "Editor" || got.JobDescription != "Edits." || got.Tier != org.TierBest ||
		got.Avatar != "owl" || got.ModelOverride != "opus-4" {
		t.Fatalf("bot = %+v", got)
	}
	if got.FolderPath == "/elsewhere" || got.DMConversationID == "" {
		t.Fatalf("UpdateBot changed the folder or lost the DM: %+v", got)
	}
	if _, err := db.UpdateBot(t.Context(), org.Bot{ID: "nope"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateBot of a missing bot: %v", err)
	}
}

func TestMessages_AttachmentsRoundTrip(t *testing.T) {
	db := newStore(t)
	dana := hire(t, db, "Dana")
	want := []org.FileRef{{Path: "/w/brief.md", Name: "brief.md"}}
	m, err := db.AppendMessage(t.Context(), org.Message{ConversationID: dana.DMConversationID, Kind: org.MessageText,
		Content: "see attached", Attachments: want})
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.Message(t.Context(), m.ID)
	if err != nil || !slices.Equal(got.Attachments, want) {
		t.Fatalf("Message = %+v, %v", got, err)
	}
	m.Attachments = nil
	if err := db.UpdateMessage(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := db.Messages(t.Context(), dana.DMConversationID, 0, 10)
	if err != nil || len(msgs) != 1 || msgs[0].Attachments != nil {
		t.Fatalf("Messages after clearing = %+v, %v", msgs, err)
	}
}
