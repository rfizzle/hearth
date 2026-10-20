package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
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
		COALESCE(run_id, ''), streaming, created_at, widget_spec, widget_answer, attachments
		FROM messages WHERE conversation_id = ?`
	args := []any{convID}
	if beforeSeq > 0 {
		query += ` AND seq < ?`
		args = append(args, beforeSeq)
	}
	query += ` ORDER BY seq DESC LIMIT ?`
	args = append(args, limit+1)

	msgs, err = sqlrow.QueryAll(ctx, db.sql, "messages", scanMessage, query, args...)
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
	parts, err := encodeParts(m)
	if err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	now := db.clock()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.ID = sqlrow.NewID()

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
		return org.Message{}, fmt.Errorf("append message: conversation %s: %w", m.ConversationID, sqlrow.OrNotFound(err))
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = ?`,
		m.ConversationID).Scan(&m.Seq); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO messages
		(id, conversation_id, seq, author_bot_id, kind, content, tool_calls, tool_call_id, files, run_id, streaming, created_at,
		widget_spec, widget_answer, attachments)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.Seq, m.AuthorBotID, m.Kind, m.Content, parts.calls, m.ToolCallID, parts.files,
		sqlrow.NullString(m.RunID), m.Streaming, sqlrow.Millis(m.CreatedAt),
		sqlrow.NullString(m.Widget), sqlrow.NullString(m.WidgetAnswer), parts.attachments); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return org.Message{}, fmt.Errorf("append message: %w", err)
	}
	return m, nil
}

// UpdateMessage replaces what a stored message says: its content, tool calls,
// the call it answers, its files, its widget and answer, and whether it is
// still streaming. Its place in the thread, author, kind and run do not
// change. It is ErrNotFound when no message has m.ID.
func (db *DB) UpdateMessage(ctx context.Context, m org.Message) error {
	parts, err := encodeParts(m)
	if err != nil {
		return fmt.Errorf("update message: %w", err)
	}
	return sqlrow.Update(ctx, db.sql, "message "+m.ID, `UPDATE messages
		SET content = ?, tool_calls = ?, tool_call_id = ?, files = ?, streaming = ?,
		widget_spec = ?, widget_answer = ?, attachments = ? WHERE id = ?`,
		m.Content, parts.calls, m.ToolCallID, parts.files, m.Streaming, sqlrow.NullString(m.Widget), sqlrow.NullString(m.WidgetAnswer), parts.attachments, m.ID)
}

// DeleteMessage removes the message with id. It is ErrNotFound when there is
// none.
func (db *DB) DeleteMessage(ctx context.Context, id string) error {
	return sqlrow.DeleteByID(ctx, db.sql, "message", "messages", id)
}

// CloseStreamingMessages ends every message still streaming: one with no
// text is deleted, since nothing of it was kept, and the rest are set to not
// streaming. It is for startup, before any run begins: a message still
// streaming then was cut off by the core stopping. It returns how many it
// closed and how many it deleted.
// See docs/model.md#9-failure-handling.
func (db *DB) CloseStreamingMessages(ctx context.Context) (closed, deleted int64, err error) {
	err = sqlrow.InTx(ctx, db.sql, "close streaming messages", func(tx *sql.Tx) error {
		for _, q := range []struct {
			n   *int64
			sql string
		}{
			{&deleted, `DELETE FROM messages WHERE streaming = 1 AND content = ''`},
			{&closed, `UPDATE messages SET streaming = 0 WHERE streaming = 1`},
		} {
			res, err := tx.ExecContext(ctx, q.sql)
			if err != nil {
				return err
			}
			if *q.n, err = res.RowsAffected(); err != nil {
				return err
			}
		}
		return nil
	})
	return closed, deleted, err
}

// jsonParts are the columns of a message that hold JSON, NULL when empty.
type jsonParts struct{ calls, files, attachments sql.NullString }

// encodeParts is m's tool calls, files and attachments as JSON.
func encodeParts(m org.Message) (p jsonParts, err error) {
	for i, enc := range []struct {
		to *sql.NullString
		v  any
		n  int
	}{
		{&p.calls, m.ToolCalls, len(m.ToolCalls)},
		{&p.files, m.Files, len(m.Files)},
		{&p.attachments, m.Attachments, len(m.Attachments)},
	} {
		if enc.n == 0 {
			continue
		}
		b, err := json.Marshal(enc.v)
		if err != nil {
			return jsonParts{}, fmt.Errorf("part %d: %w", i, err)
		}
		*enc.to = sql.NullString{String: string(b), Valid: true}
	}
	return p, nil
}

func scanMessage(row sqlrow.Scanner) (org.Message, error) {
	var m org.Message
	var p jsonParts
	var widget, answer sql.NullString
	var created sql.NullInt64
	if err := row.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.AuthorBotID, &m.Kind, &m.Content, &p.calls,
		&m.ToolCallID, &p.files, &m.RunID, &m.Streaming, &created, &widget, &answer, &p.attachments); err != nil {
		return org.Message{}, err
	}
	m.CreatedAt = sqlrow.FromMillis(created)
	m.Widget, m.WidgetAnswer = widget.String, answer.String
	err := decodeFields("message "+m.ID, jsonField{p.calls, &m.ToolCalls, "tool calls"},
		jsonField{p.files, &m.Files, "files"}, jsonField{p.attachments, &m.Attachments, "attachments"})
	if err != nil {
		return org.Message{}, err
	}
	return m, nil
}

// encodeJSON is v as JSON, or NULL when n, its length, is zero.
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
