package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// Message is the message with id, or ErrNotFound.
func (db *DB) Message(ctx context.Context, id string) (org.Message, error) {
	m, err := scanMessage(db.sql.QueryRowContext(ctx, `SELECT id, conversation_id, seq, author_bot_id, kind,
		content, tool_calls, tool_call_id, files, COALESCE(run_id, ''), streaming, created_at, widget_spec,
		widget_answer FROM messages WHERE id = ?`, id))
	if err != nil {
		return org.Message{}, fmt.Errorf("message %s: %w", id, notFound(err))
	}
	return m, nil
}
