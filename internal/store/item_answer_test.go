package store

import (
	"errors"
	"testing"
)

func TestAnswerInboxItem(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Dana")
	ctx := t.Context()
	it, err := db.CreateInboxItem(ctx, approvalFor(bot))
	if err != nil {
		t.Fatal(err)
	}
	const answer = `{"kind":"yes_no","yes":true}`
	if err := db.AnswerInboxItem(ctx, it.ID, answer); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.InboxItem(ctx, it.ID); got.WidgetAnswer != answer {
		t.Fatalf("widget answer = %q, want %q", got.WidgetAnswer, answer)
	}
	if err := db.CloseInboxItem(ctx, it.ID, 0, "Yes", "Answered"); err != nil {
		t.Fatal(err)
	}
	if err := db.AnswerInboxItem(ctx, it.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("answering a closed item = %v, want ErrNotFound", err)
	}
	if got, _ := db.InboxItem(ctx, it.ID); got.WidgetAnswer != answer {
		t.Fatal("a closed item's answer changed")
	}
}
