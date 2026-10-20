package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
)

func TestMessage_ReadsOneByID(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Asker")
	m, err := db.AppendMessage(t.Context(), org.Message{
		ConversationID: bot.DMConversationID, AuthorBotID: bot.ID, Kind: org.MessageQuestion,
		Content: "Which one?", ToolCalls: []org.ToolCall{{ID: "c1", Name: "ask_owner"}},
		Widget: `{"kind":"yes_no"}`, WidgetAnswer: `{"kind":"yes_no","yes":true}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.Message(t.Context(), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != m.ID || got.Seq != m.Seq || got.Kind != org.MessageQuestion || got.Content != m.Content ||
		got.Widget != m.Widget || got.WidgetAnswer != m.WidgetAnswer || !slices.Equal(got.ToolCalls, m.ToolCalls) {
		t.Fatalf("Message = %+v\nwant      %+v", got, m)
	}
	if _, err := db.Message(t.Context(), "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Message(gone) = %v, want ErrNotFound", err)
	}
}
