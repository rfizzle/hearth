package store

import (
	"errors"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func TestConversation_CarriesItsNewestRun(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Dana")
	ctx := t.Context()

	c, err := db.Conversation(ctx, bot.DMConversationID)
	if err != nil || c.BotID != bot.ID || c.LatestRun != nil {
		t.Fatalf("Conversation before a run = %+v, %v, want the DM with no run", c, err)
	}
	first, err := db.CreateRun(ctx, org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunDone})
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateRun(ctx, org.Run{ConversationID: bot.DMConversationID, BotID: bot.ID, State: org.RunFailed, Failure: "no model"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = db.Conversation(ctx, bot.DMConversationID)
	if err != nil || c.LatestRun == nil || c.LatestRun.ID != second.ID || c.LatestRun.ID == first.ID || c.LatestRun.Failure != "no model" {
		t.Fatalf("Conversation = %+v, %v, want the newest run", c, err)
	}
	if _, err := db.Conversation(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Conversation of a missing id: %v, want ErrNotFound", err)
	}
}
