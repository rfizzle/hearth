package store

import (
	"context"

	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// AnswerInboxItem records answer, the JSON of the owner's answer to the
// control of the open item id, or none when empty. It is ErrNotFound when
// no open item has id.
func (db *DB) AnswerInboxItem(ctx context.Context, id, answer string) error {
	return sqlrow.Update(ctx, db.sql, "inbox item "+id, `UPDATE inbox_items SET widget_answer = ? WHERE id = ? AND state = ?`,
		sqlrow.NullString(answer), id, inbox.Open)
}
