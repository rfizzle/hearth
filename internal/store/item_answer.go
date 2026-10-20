package store

import (
	"context"

	"github.com/rfizzle/hearth/internal/org/inbox"
)

// AnswerInboxItem records answer, the JSON of the owner's answer to the
// control of the open item id, or none when empty. It is ErrNotFound when
// no open item has id.
func (db *DB) AnswerInboxItem(ctx context.Context, id, answer string) error {
	return db.update(ctx, "inbox item "+id, `UPDATE inbox_items SET widget_answer = ? WHERE id = ? AND state = ?`,
		nullString(answer), id, inbox.Open)
}
