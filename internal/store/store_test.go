package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
)

var orgFixture = org.Org{Name: "Acme", WorkspacePath: "/Users/me/Hearth"}

// open opens a store in path through Open, so the migrations run, and closes
// it when the test ends.
func open(t *testing.T, path string) *DB {
	t.Helper()
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newStore(t *testing.T) *DB {
	t.Helper()
	return open(t, filepath.Join(t.TempDir(), "hearth.db"))
}

func hire(t *testing.T, db *DB, name string) org.Bot {
	t.Helper()
	bot, grants, dm := org.NewBot(name, "Analyst", "Finds things out.", org.TierBalanced, orgFixture.WorkspacePath)
	stored, err := db.CreateBot(t.Context(), bot, grants, dm)
	if err != nil {
		t.Fatalf("CreateBot: %v", err)
	}
	return stored
}

func TestOrg(t *testing.T) {
	db := newStore(t)
	if _, err := db.Org(t.Context()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Org before PutOrg: %v, want ErrNotFound", err)
	}
	for _, o := range []org.Org{orgFixture, {Name: "Acme Two", WorkspacePath: "/elsewhere"}} {
		if err := db.PutOrg(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		if got, err := db.Org(t.Context()); err != nil || got != o {
			t.Fatalf("Org = %+v, %v, want %+v", got, err, o)
		}
	}
}

func TestCreateBot_RefusesAFolderAnotherBotHasWhateverTheCase(t *testing.T) {
	db := newStore(t)
	hire(t, db, "Dana")
	for _, name := range []string{"Dana", "dana", "DANA"} {
		bot, grants, dm := org.NewBot(name, "Writer", "Writes.", org.TierFast, orgFixture.WorkspacePath)
		if _, err := db.CreateBot(t.Context(), bot, grants, dm); !errors.Is(err, ErrFolderTaken) {
			t.Errorf("CreateBot(%q) = %v, want ErrFolderTaken", name, err)
		}
	}
	if bots, err := db.Bots(t.Context()); err != nil || len(bots) != 1 {
		t.Fatalf("Bots after refusals = %d, %v, want 1", len(bots), err)
	}
	hire(t, db, "Dan")
}

func TestCreateBot_StoresTheBotItsGrantsAndItsDM(t *testing.T) {
	db := newStore(t)
	db.now = func() time.Time { return time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC) }
	bot := hire(t, db, "Research Analyst")
	if bot.ID == "" || bot.DMConversationID == "" {
		t.Fatalf("CreateBot minted no ids: %+v", bot)
	}

	got, err := db.Bot(t.Context(), bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != bot {
		t.Fatalf("Bot = %+v, want %+v", got, bot)
	}
	bots, err := db.Bots(t.Context())
	if err != nil || len(bots) != 1 || bots[0] != bot {
		t.Fatalf("Bots = %+v, %v", bots, err)
	}

	_, wantGrants, _ := org.NewBot("Research Analyst", "", "", org.TierBalanced, orgFixture.WorkspacePath)
	for i := range wantGrants {
		wantGrants[i].BotID = bot.ID
	}
	grants, err := db.GrantsFor(t.Context(), bot.ID)
	if err != nil || !slices.Equal(grants, wantGrants) {
		t.Fatalf("GrantsFor = %+v, %v, want %+v", grants, err, wantGrants)
	}

	convs, err := db.Conversations(t.Context(), bot.ID)
	if err != nil || len(convs) != 1 {
		t.Fatalf("Conversations = %+v, %v", convs, err)
	}
	if c := convs[0]; c.ID != bot.DMConversationID || c.Kind != org.ConversationDM || c.BotID != bot.ID || c.Title != "Research Analyst" {
		t.Fatalf("DM = %+v", c)
	}

	if _, err := db.Bot(t.Context(), "nobody"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Bot(nobody): %v, want ErrNotFound", err)
	}
}

// A grant that cannot be stored leaves no bot and no DM behind.
func TestCreateBot_IsOneTransaction(t *testing.T) {
	db := newStore(t)
	bot, grants, dm := org.NewBot("Writer", "", "", org.TierFast, "/Hearth")
	dm.Kind = 0
	if _, err := db.sql.Exec(`CREATE TRIGGER refuse BEFORE INSERT ON conversations BEGIN SELECT RAISE(ABORT, 'refused'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateBot(t.Context(), bot, grants, dm); err == nil {
		t.Fatal("CreateBot succeeded with the DM refused")
	}
	var bots, rows int
	if err := db.sql.QueryRow(`SELECT (SELECT COUNT(*) FROM bots), (SELECT COUNT(*) FROM grants)`).Scan(&bots, &rows); err != nil {
		t.Fatal(err)
	}
	if bots != 0 || rows != 0 {
		t.Fatalf("after a failed CreateBot: %d bots, %d grants", bots, rows)
	}
}

func TestMessages_PagesBackThroughAConversation(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Writer")
	for i := 1; i <= 120; i++ {
		m, err := db.AppendMessage(t.Context(), org.Message{
			ConversationID: bot.DMConversationID,
			Kind:           org.MessageText,
			Content:        fmt.Sprintf("message %d", i),
		})
		if err != nil {
			t.Fatal(err)
		}
		if m.Seq != int64(i) || m.ID == "" {
			t.Fatalf("message %d stored with seq %d, id %q", i, m.Seq, m.ID)
		}
	}

	var seen []int64
	var sizes []int
	before := int64(0)
	for {
		page, more, err := db.Messages(t.Context(), bot.DMConversationID, before, 50)
		if err != nil {
			t.Fatal(err)
		}
		sizes = append(sizes, len(page))
		var seqs []int64
		for _, m := range page {
			seqs = append(seqs, m.Seq)
		}
		if !slices.IsSorted(seqs) {
			t.Fatalf("page is not oldest first: %v", seqs)
		}
		seen = append(seqs, seen...)
		if !more {
			break
		}
		before = page[0].Seq
	}
	if !slices.Equal(sizes, []int{50, 50, 20}) {
		t.Fatalf("page sizes = %v, want [50 50 20]", sizes)
	}
	for i, seq := range seen {
		if seq != int64(i+1) {
			t.Fatalf("walk saw seq %d at position %d; every message once, in order, is wanted", seq, i)
		}
	}
}

func TestAppendMessage_ToAMissingConversation(t *testing.T) {
	db := newStore(t)
	_, err := db.AppendMessage(t.Context(), org.Message{ConversationID: "gone", Kind: org.MessageText})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("AppendMessage: %v, want ErrNotFound", err)
	}
}

func TestUpdateMessage_RoundTripsToolCallsAndFiles(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Writer")
	run, err := db.CreateRun(t.Context(), org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunWorking})
	if err != nil {
		t.Fatal(err)
	}
	m, err := db.AppendMessage(t.Context(), org.Message{
		ConversationID: bot.DMConversationID,
		AuthorBotID:    bot.ID,
		Kind:           org.MessageToolCall,
		RunID:          run.ID,
		Streaming:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	m.Content = "Writing the report."
	m.ToolCalls = []org.ToolCall{{ID: "c1", Name: "write", Arguments: `{"path":"report.md"}`, Summary: "Write report.md"}}
	m.ToolCallID = "c0"
	m.Files = []org.FileRef{{Path: "/Hearth/Writer/report.md", Name: "report.md"}}
	m.Streaming = false
	if err := db.UpdateMessage(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	page, _, err := db.Messages(t.Context(), bot.DMConversationID, 0, 0)
	if err != nil || len(page) != 1 {
		t.Fatalf("Messages = %+v, %v", page, err)
	}
	got := page[0]
	if got.ID != m.ID || got.Content != m.Content || got.ToolCallID != "c0" || got.Streaming || got.RunID != run.ID ||
		got.AuthorBotID != bot.ID || got.Kind != org.MessageToolCall || !got.CreatedAt.Equal(m.CreatedAt) ||
		!slices.Equal(got.ToolCalls, m.ToolCalls) || !slices.Equal(got.Files, m.Files) {
		t.Fatalf("stored = %+v\nwant     %+v", got, m)
	}
	if err := db.UpdateMessage(t.Context(), org.Message{ID: "gone"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateMessage(gone): %v, want ErrNotFound", err)
	}
}

func TestRuns(t *testing.T) {
	db := newStore(t)
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	db.now = func() time.Time { return at }
	bot := hire(t, db, "Planner")

	run, err := db.CreateRun(t.Context(), org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunQueued})
	if err != nil || run.ID == "" {
		t.Fatalf("CreateRun = %+v, %v", run, err)
	}
	run.State, run.Steps, run.StartedAt = org.RunWorking, 3, at.Add(-time.Minute)
	if err := db.UpdateRun(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Run(t.Context(), run.ID); err != nil || got != run {
		t.Fatalf("Run = %+v, %v, want %+v", got, err, run)
	}

	done, err := db.CreateRun(t.Context(), org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunDone})
	if err != nil {
		t.Fatal(err)
	}
	queued, err := db.CreateRun(t.Context(), org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunQueued})
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.FailWorkingRuns(t.Context(), "hearth was quit while this was running")
	if err != nil || n != 2 {
		t.Fatalf("FailWorkingRuns = %d, %v, want 2", n, err)
	}
	for _, id := range []string{run.ID, queued.ID} {
		got, err := db.Run(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != org.RunFailed || got.Failure != "hearth was quit while this was running" || !got.EndedAt.Equal(at) {
			t.Fatalf("run %s after the sweep = %+v", id, got)
		}
	}
	if got, err := db.Run(t.Context(), done.ID); err != nil || got.State != org.RunDone || got.Failure != "" {
		t.Fatalf("a finished run was swept: %+v, %v", got, err)
	}

	if _, err := db.Run(t.Context(), "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Run(gone): %v, want ErrNotFound", err)
	}
	if err := db.UpdateRun(t.Context(), org.Run{ID: "gone"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateRun(gone): %v, want ErrNotFound", err)
	}
}
