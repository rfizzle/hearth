package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/rfizzle/hearth/internal/org"
)

// Page sizes for Messages: the size a limit of zero means, and the most one
// call returns.
const (
	defaultPage = 50
	maxPage     = 200
)

// Messages is a page of the conversation convID, oldest first: the limit
// messages just before beforeSeq, or the newest when beforeSeq is zero. more
// is true when older messages remain, and the next page asks for the ones
// before the first message of this one. A limit of zero or less is 50, and
// one over 200 is 200.
func (db *DB) Messages(ctx context.Context, convID string, beforeSeq int64, limit int) (msgs []org.Message, more bool, err error) {
	if limit <= 0 {
		limit = defaultPage
	}
	limit = min(limit, maxPage)
	query := `SELECT id, conversation_id, seq, author_bot_id, kind, content, tool_calls, tool_call_id, files,
		COALESCE(run_id, ''), streaming, created_at
		FROM messages WHERE conversation_id = ?`
	args := []any{convID}
	if beforeSeq > 0 {
		query += ` AND seq < ?`
		args = append(args, beforeSeq)
	}
	query += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit+1)

	msgs, err = queryAll(ctx, db.sql, "messages", scanMessage, query, args...)
	if err != nil {
		return nil, false, err
	}
	if len(msgs) > limit {
		msgs, more = msgs[:limit], true
	}
	slices.Reverse(msgs)
	return msgs, more, nil
}

// AppendMessage stores m at the end of its conversation, minting its id and
// assigning the next seq, and returns it as stored. It is ErrNotFound when
// the conversation does not exist.
func (db *DB) AppendMessage(ctx context.Context, m org.Message) (org.Message, error) {
	calls, files, err := encodeParts(m)
	if err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	now := db.clock()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.ID = newID()

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `UPDATE conversations SET updated_at = ? WHERE id = ?`, now.UnixMilli(), m.ConversationID)
	if err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return org.Message{}, fmt.Errorf("append message: conversation %s: %w", m.ConversationID, orNotFound(err))
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = ?`,
		m.ConversationID).Scan(&m.Seq); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO messages
		(id, conversation_id, seq, author_bot_id, kind, content, tool_calls, tool_call_id, files, run_id, streaming, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.Seq, m.AuthorBotID, m.Kind, m.Content, calls, m.ToolCallID, files,
		nullString(m.RunID), m.Streaming, millis(m.CreatedAt)); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	return m, nil
}

// UpdateMessage replaces what a stored message says: its content, tool calls,
// the call it answers, its files and whether it is still streaming. Its
// place in the thread, author, kind and run do not change. It is ErrNotFound
// when no message has m.ID.
func (db *DB) UpdateMessage(ctx context.Context, m org.Message) error {
	calls, files, err := encodeParts(m)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}
	return db.update(ctx, "message "+m.ID, `UPDATE messages
		SET content = ?, tool_calls = ?, tool_call_id = ?, files = ?, streaming = ? WHERE id = ?`,
		m.Content, calls, m.ToolCallID, files, m.Streaming, m.ID)
}

// encodeParts is m's tool calls and files as JSON, NULL when there are none.
func encodeParts(m org.Message) (calls, files sql.NullString, err error) {
	if calls, err = encodeJSON(m.ToolCalls, len(m.ToolCalls)); err != nil {
		return calls, files, err
	}
	files, err = encodeJSON(m.Files, len(m.Files))
	return calls, files, err
}

func encodeJSON(v any, n int) (sql.NullString, error) {
	if n == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func scanMessage(row scanner) (org.Message, error) {
	var m org.Message
	var calls, files sql.NullString
	var created sql.NullInt64
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.AuthorBotID, &m.Kind, &m.Content, &calls,
		&m.ToolCallID, &files, &m.RunID, &m.Streaming, &created); err != nil {
		return org.Message{}, err
	}
	m.CreatedAt = fromMillis(created)
	if calls.Valid {
		if err := json.Unmarshal([]byte(calls.String), &m.ToolCalls); err != nil {
			return org.Message{}, fmt.Errorf("message %s tool calls: %w", m.ID, err)
		}
	}
	if files.Valid {
		if err := json.Unmarshal([]byte(files.String), &m.Files); err != nil {
			return org.Message{}, fmt.Errorf("message %s files: %w", m.ID, err)
		}
	}
	return m, nil
}
