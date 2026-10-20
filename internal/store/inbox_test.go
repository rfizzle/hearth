package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

func approvalFor(bot org.Bot) inbox.Item {
	return inbox.Item{
		Kind:           inbox.Approval,
		BotID:          bot.ID,
		ConversationID: bot.DMConversationID,
		Title:          "Delete a file",
		Effect:         "Delete brief.md from Dana's folder",
		Call:           &org.ToolCall{ID: "c1", Name: "files.delete", Arguments: `{"path":"brief.md"}`},
	}
}

func TestInboxItems_CreateListCloseAndRead(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Dana")
	ctx := t.Context()

	item, err := db.CreateInboxItem(ctx, approvalFor(bot))
	if err != nil {
		t.Fatalf("CreateInboxItem: %v", err)
	}
	if item.ID == "" || item.State != inbox.Open || item.CreatedAt.IsZero() {
		t.Fatalf("created item = %+v", item)
	}

	got, err := db.InboxItem(ctx, item.ID)
	if err != nil || got.Call == nil || *got.Call != *item.Call || got.Effect != item.Effect || got.ConversationID != bot.DMConversationID {
		t.Fatalf("InboxItem = %+v, %v, want %+v", got, err, item)
	}
	if _, err := db.InboxItem(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("InboxItem of a missing id: %v, want ErrNotFound", err)
	}

	open, err := db.InboxItems(ctx, true)
	if err != nil || len(open) != 1 || open[0].ID != item.ID {
		t.Fatalf("open items = %+v, %v", open, err)
	}

	if err := db.CloseInboxItem(ctx, item.ID, inbox.Deny, "", "Declined"); err != nil {
		t.Fatalf("CloseInboxItem: %v", err)
	}
	if open, err = db.InboxItems(ctx, true); err != nil || len(open) != 0 {
		t.Fatalf("open items after closing = %+v, %v", open, err)
	}
	all, err := db.InboxItems(ctx, false)
	if err != nil || len(all) != 1 {
		t.Fatalf("all items = %+v, %v", all, err)
	}
	c := all[0]
	if c.State != inbox.Closed || c.Decision != inbox.Deny || c.Outcome != "Declined" || c.ClosedAt.IsZero() {
		t.Fatalf("closed item = %+v", c)
	}
	if err := db.CloseInboxItem(ctx, item.ID, inbox.AllowOnce, "", "again"); !errors.Is(err, ErrItemClosed) {
		t.Fatalf("closing twice: %v, want ErrItemClosed", err)
	}
	if err := db.CloseInboxItem(ctx, "nope", inbox.Deny, "", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("closing a missing item: %v, want ErrNotFound", err)
	}
}

func TestOpenItemFor_FindsTheOneOpenItemAndRefusesASecond(t *testing.T) {
	db := newStore(t)
	dana, sam := hire(t, db, "Dana"), hire(t, db, "Sam")
	ctx := t.Context()

	if _, ok, err := db.OpenItemFor(ctx, dana.DMConversationID); err != nil || ok {
		t.Fatalf("OpenItemFor with none = %v, %v", ok, err)
	}
	q := approvalFor(dana)
	q.Kind, q.Call, q.Question, q.Suggested = inbox.Question, nil, "Which one?", []string{"A", "B"}
	q.Widget = `{"kind":"yes_no"}`
	first, err := db.CreateInboxItem(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateInboxItem(ctx, approvalFor(dana)); !errors.Is(err, ErrItemOpen) {
		t.Fatalf("a second open item: %v, want ErrItemOpen", err)
	}
	if _, err := db.CreateInboxItem(ctx, approvalFor(sam)); err != nil {
		t.Fatalf("an item in another conversation: %v", err)
	}

	got, ok, err := db.OpenItemFor(ctx, dana.DMConversationID)
	if err != nil || !ok || got.ID != first.ID || !slices.Equal(got.Suggested, q.Suggested) || got.Widget != q.Widget || got.Call != nil {
		t.Fatalf("OpenItemFor = %+v, %v, %v", got, ok, err)
	}

	if err := db.CloseInboxItem(ctx, first.ID, 0, "A", "Answered"); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := db.OpenItemFor(ctx, dana.DMConversationID); err != nil || ok {
		t.Fatalf("OpenItemFor after closing = %v, %v", ok, err)
	}
	if _, err := db.CreateInboxItem(ctx, approvalFor(dana)); err != nil {
		t.Fatalf("a new item once the first is closed: %v", err)
	}
}

func TestAllowances_ByConversationAndCapability(t *testing.T) {
	db := newStore(t)
	dana, sam := hire(t, db, "Dana"), hire(t, db, "Sam")
	ctx := t.Context()

	a := inbox.Allowance{ConversationID: dana.DMConversationID, Capability: org.FilesWrite, Folder: "/w/Dana"}
	for range 2 {
		if err := db.AddAllowance(ctx, a); err != nil {
			t.Fatalf("AddAllowance: %v", err)
		}
	}
	b := inbox.Allowance{ConversationID: dana.DMConversationID, Capability: org.FilesDelete, Folder: "/w/Dana"}
	if err := db.AddAllowance(ctx, b); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Allowances(ctx, dana.DMConversationID); err != nil || !slices.Equal(got, []inbox.Allowance{a, b}) {
		t.Fatalf("Allowances = %+v, %v, want %+v", got, err, []inbox.Allowance{a, b})
	}
	if got, err := db.Allowances(ctx, sam.DMConversationID); err != nil || len(got) != 0 {
		t.Fatalf("another conversation's allowances = %+v, %v", got, err)
	}
	if err := db.AddAllowance(ctx, inbox.Allowance{ConversationID: "nope", Capability: org.FilesWrite}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("AddAllowance to a missing conversation: %v, want ErrNotFound", err)
	}
}

func TestReplaceGrants_KeepsTheOwnFolderAndReplacesTheRest(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Dana")
	other := hire(t, db, "Sam")
	ctx := t.Context()
	before, err := db.GrantsFor(ctx, bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherBefore, _ := db.GrantsFor(ctx, other.ID)

	req := []org.Grant{
		{Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: "/w/Shared"},
		{Capability: org.FilesWrite, Mode: org.ModeNotGranted, Folder: bot.FolderPath}, // tries to drop the own folder
		{Capability: org.WebFetch, Mode: org.ModeAsk},
		{Capability: org.AskOwner, Mode: org.ModeNotGranted}, // tries to drop the right to ask
	}
	if err := db.ReplaceGrants(ctx, bot.ID, req); err != nil {
		t.Fatalf("ReplaceGrants: %v", err)
	}
	got, err := db.GrantsFor(ctx, bot.ID)
	if err != nil {
		t.Fatal(err)
	}
	var own, rest []org.Grant
	for _, g := range got {
		if g.Folder == bot.FolderPath {
			own = append(own, g)
		} else {
			rest = append(rest, g)
		}
	}
	var wantOwn []org.Grant
	for _, g := range before {
		if g.Folder == bot.FolderPath {
			wantOwn = append(wantOwn, g)
		}
	}
	if len(wantOwn) != 3 || !slices.Equal(own, wantOwn) {
		t.Fatalf("own-folder grants = %+v, want %+v", own, wantOwn)
	}
	wantRest := []org.Grant{
		{BotID: bot.ID, Capability: org.AskOwner, Mode: org.ModeAllowed}, // kept: the owner never edits it
		{BotID: bot.ID, Capability: org.FilesRead, Mode: org.ModeAllowed, Folder: "/w/Shared"},
		{BotID: bot.ID, Capability: org.WebFetch, Mode: org.ModeAsk},
	}
	if !slices.Equal(rest, wantRest) {
		t.Fatalf("other grants = %+v, want %+v", rest, wantRest)
	}
	if after, _ := db.GrantsFor(ctx, other.ID); !slices.Equal(after, otherBefore) {
		t.Fatalf("another bot's grants changed: %+v", after)
	}
	if err := db.ReplaceGrants(ctx, "nope", req); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ReplaceGrants of a missing bot: %v, want ErrNotFound", err)
	}
}

func TestMessages_RoundTripTheWidgetAndItsAnswer(t *testing.T) {
	db := newStore(t)
	bot := hire(t, db, "Dana")
	ctx := t.Context()

	plain, err := db.AppendMessage(ctx, org.Message{ConversationID: bot.DMConversationID, Kind: org.MessageText, Content: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := db.AppendMessage(ctx, org.Message{ConversationID: bot.DMConversationID, Kind: org.MessageText,
		AuthorBotID: bot.ID, Content: "Which?", Widget: `{"kind":"yes_no"}`})
	if err != nil {
		t.Fatal(err)
	}
	q.WidgetAnswer = `{"yes":true}`
	if err := db.UpdateMessage(ctx, q); err != nil {
		t.Fatal(err)
	}

	msgs, _, err := db.Messages(ctx, bot.DMConversationID, 0, 10)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("Messages = %+v, %v", msgs, err)
	}
	if msgs[0].ID != plain.ID || msgs[0].Widget != "" || msgs[0].WidgetAnswer != "" {
		t.Fatalf("plain message = %+v", msgs[0])
	}
	if msgs[1].Widget != `{"kind":"yes_no"}` || msgs[1].WidgetAnswer != `{"yes":true}` {
		t.Fatalf("widget message = %+v", msgs[1])
	}
}
