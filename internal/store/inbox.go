package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
)

// ErrItemOpen is returned when a conversation already has an open Inbox item:
// a conversation waits on one thing at a time.
var ErrItemOpen = errors.New("the conversation already has an open item")

// ErrItemClosed is returned when an item is already closed.
var ErrItemClosed = errors.New("the item is already closed")

const itemColumns = `SELECT id, kind, state, bot_id, COALESCE(conversation_id, ''), COALESCE(run_id, ''),
	COALESCE(message_id, ''), title, effect, call, question, suggested, widget_spec, widget_answer,
	created_at, closed_at, decision, answer, outcome FROM inbox_items`

// CreateInboxItem stores a new open item, minting its id and stamping its
// time, and returns it as stored. An item with a conversation is
// ErrItemOpen when that conversation already has an open one.
// See docs/model.md#8-approvals.
func (db *DB) CreateInboxItem(ctx context.Context, it inbox.Item) (inbox.Item, error) {
	var call sql.NullString
	if it.Call != nil {
		b, err := json.Marshal(it.Call)
		if err != nil {
			return inbox.Item{}, fmt.Errorf("create inbox item: %w", err)
		}
		call = sql.NullString{String: string(b), Valid: true}
	}
	suggested, err := encodeJSON(it.Suggested, len(it.Suggested))
	if err != nil {
		return inbox.Item{}, fmt.Errorf("create inbox item: %w", err)
	}
	it.ID = newID()
	it.State = inbox.Open
	it.CreatedAt = db.clock()
	it.ClosedAt = time.Time{}
	it.Decision, it.Answer, it.Outcome = 0, "", ""

	// The unique index on open items per conversation is what holds the rule;
	// a second insert is refused there, however the two calls interleave.
	if _, err := db.sql.ExecContext(ctx, `INSERT INTO inbox_items
		(id, kind, state, bot_id, conversation_id, run_id, message_id, title, effect, call, question, suggested,
		widget_spec, widget_answer, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		it.ID, it.Kind, it.State, it.BotID, nullString(it.ConversationID), nullString(it.RunID),
		nullString(it.MessageID), it.Title, it.Effect, call, it.Question, suggested,
		nullString(it.Widget), nullString(it.WidgetAnswer), it.CreatedAt.UnixMilli()); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: inbox_items.conversation_id") {
			return inbox.Item{}, fmt.Errorf("create inbox item: conversation %s: %w", it.ConversationID, ErrItemOpen)
		}
		return inbox.Item{}, fmt.Errorf("create inbox item: %w", err)
	}
	return it, nil
}

// InboxItems is the items in the Inbox, oldest first: only the open ones
// when openOnly, otherwise the closed ones too.
func (db *DB) InboxItems(ctx context.Context, openOnly bool) ([]inbox.Item, error) {
	query := itemColumns
	var args []any
	if openOnly {
		query += ` WHERE state = ?`
		args = append(args, inbox.Open)
	}
	query += ` ORDER BY created_at, rowid`
	return queryAll(ctx, db.sql, "inbox items", scanItem, query, args...)
}

// InboxItem is the item with id, or ErrNotFound.
func (db *DB) InboxItem(ctx context.Context, id string) (inbox.Item, error) {
	it, err := scanItem(db.sql.QueryRowContext(ctx, itemColumns+` WHERE id = ?`, id))
	if err != nil {
		return inbox.Item{}, fmt.Errorf("inbox item %s: %w", id, notFound(err))
	}
	return it, nil
}

// OpenItemFor is the open item the conversation waits on, and false when it
// waits on none.
func (db *DB) OpenItemFor(ctx context.Context, conversationID string) (inbox.Item, bool, error) {
	it, err := scanItem(db.sql.QueryRowContext(ctx, itemColumns+` WHERE conversation_id = ? AND state = ?`,
		conversationID, inbox.Open))
	if errors.Is(err, sql.ErrNoRows) {
		return inbox.Item{}, false, nil
	}
	if err != nil {
		return inbox.Item{}, false, fmt.Errorf("open item for %s: %w", conversationID, err)
	}
	return it, true, nil
}

// CloseInboxItem closes the open item id with the owner's decision, the
// answer they gave and a plain-words outcome. It is ErrNotFound when there is
// no such item and ErrItemClosed when it is closed already.
func (db *DB) CloseInboxItem(ctx context.Context, id string, decision inbox.Decision, answer, outcome string) error {
	res, err := db.sql.ExecContext(ctx, `UPDATE inbox_items
		SET state = ?, closed_at = ?, decision = ?, answer = ?, outcome = ? WHERE id = ? AND state = ?`,
		inbox.Closed, db.clock().UnixMilli(), decision, answer, outcome, id, inbox.Open)
	if err != nil {
		return fmt.Errorf("close inbox item %s: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("close inbox item %s: %w", id, err)
	}
	if n == 0 {
		if _, err := db.InboxItem(ctx, id); err != nil {
			return fmt.Errorf("close inbox item: %w", err)
		}
		return fmt.Errorf("close inbox item %s: %w", id, ErrItemClosed)
	}
	return nil
}

func scanItem(row scanner) (inbox.Item, error) {
	var it inbox.Item
	var call, suggested, widget, answer sql.NullString
	var created, closed sql.NullInt64
	if err := row.Scan(&it.ID, &it.Kind, &it.State, &it.BotID, &it.ConversationID, &it.RunID, &it.MessageID,
		&it.Title, &it.Effect, &call, &it.Question, &suggested, &widget, &answer, &created, &closed,
		&it.Decision, &it.Answer, &it.Outcome); err != nil {
		return inbox.Item{}, err
	}
	it.CreatedAt, it.ClosedAt = fromMillis(created), fromMillis(closed)
	it.Widget, it.WidgetAnswer = widget.String, answer.String
	if call.Valid {
		it.Call = new(org.ToolCall)
	}
	err := decodeFields("item "+it.ID, jsonField{call, it.Call, "call"},
		jsonField{suggested, &it.Suggested, "suggested answers"})
	if err != nil {
		return inbox.Item{}, err
	}
	return it, nil
}
