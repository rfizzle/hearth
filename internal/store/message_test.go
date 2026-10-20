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
