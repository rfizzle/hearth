package store

import (
	"errors"
	"testing"
	"time"

	"github.com/rfizzle/hearth/internal/org"
)

func TestFindOrg_NoOrgIsEmptyNotAnError(t *testing.T) {
	db := newStore(t)
	if got, ok, err := db.FindOrg(t.Context()); err != nil || ok || got != (org.Org{}) {
		t.Fatalf("FindOrg before PutOrg = %+v, %v, %v, want empty, false, nil", got, ok, err)
	}
	if err := db.PutOrg(t.Context(), orgFixture); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := db.FindOrg(t.Context()); err != nil || !ok || got != orgFixture {
		t.Fatalf("FindOrg = %+v, %v, %v, want the org", got, ok, err)
	}
}

func TestDeleteMessage(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Planner")
	m, err := db.AppendMessage(t.Context(), org.Message{ConversationID: bot.DMConversationID, Kind: org.MessageText, Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMessage(t.Context(), m.ID); err != nil {
		t.Fatal(err)
	}
	if msgs, _, err := db.Messages(t.Context(), bot.DMConversationID, 0, 0); err != nil || len(msgs) != 0 {
		t.Fatalf("Messages = %+v, %v, want none", msgs, err)
	}
	if err := db.DeleteMessage(t.Context(), m.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteMessage twice: %v, want ErrNotFound", err)
	}
}

func TestCloseStreamingMessages(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Planner")
	conv := bot.DMConversationID
	add := func(m org.Message) org.Message {
		t.Helper()
		m.ConversationID, m.Kind = conv, org.MessageText
		got, err := db.AppendMessage(t.Context(), m)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	empty := add(org.Message{AuthorBotID: bot.ID, Streaming: true})
	partial := add(org.Message{AuthorBotID: bot.ID, Streaming: true, Content: "half a thought"})
	whole := add(org.Message{AuthorBotID: bot.ID, Content: "finished"})

	closed, deleted, err := db.CloseStreamingMessages(t.Context())
	if err != nil || closed != 1 || deleted != 1 {
		t.Fatalf("CloseStreamingMessages = %d closed, %d deleted, %v, want 1 and 1", closed, deleted, err)
	}
	msgs, _, err := db.Messages(t.Context(), conv, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].ID != partial.ID || msgs[0].Streaming || msgs[0].Content != partial.Content || msgs[1].ID != whole.ID {
		t.Fatalf("messages = %+v, want the partial closed and the empty %s gone", msgs, empty.ID)
	}
}

func TestLatestRun(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Planner")
	conv := bot.DMConversationID
	if _, ok, err := db.LatestRun(t.Context(), conv); err != nil || ok {
		t.Fatalf("LatestRun with no runs = %v, %v, want false", ok, err)
	}
	convs, err := db.Conversations(t.Context(), bot.ID)
	if err != nil || len(convs) != 1 || convs[0].LatestRun != nil {
		t.Fatalf("Conversations with no runs = %+v, %v, want no latest run", convs, err)
	}

	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	first, err := db.CreateRun(t.Context(), org.Run{ConversationID: conv, BotID: bot.ID, State: org.RunFailed, Failure: "it broke", StartedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateRun(t.Context(), org.Run{ConversationID: conv, BotID: bot.ID, State: org.RunQueued})
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.LatestRun(t.Context(), conv)
	if err != nil || !ok || got != second {
		t.Fatalf("LatestRun = %+v, %v, %v, want %+v", got, ok, err, second)
	}
	convs, err = db.Conversations(t.Context(), bot.ID)
	if err != nil || len(convs) != 1 || convs[0].LatestRun == nil || *convs[0].LatestRun != second {
		t.Fatalf("Conversations = %+v, %v, want latest run %+v (not %+v)", convs, err, second, first)
	}
}
