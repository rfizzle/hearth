package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// Conversations is every conversation the bot with botID is in, the most
// recently active first, each with its newest run when it has had one.
func (db *DB) Conversations(ctx context.Context, botID string) ([]org.Conversation, error) {
	return queryAll(ctx, db.sql, "conversations", scanConversation, conversationSelect+
		` WHERE c.bot_id = ? ORDER BY c.updated_at DESC, c.rowid DESC`, botID)
}

// conversationSelect reads a conversation and its newest run; a caller adds
// the WHERE.
const conversationSelect = `SELECT c.id, c.kind, COALESCE(c.bot_id, ''), c.title, c.created_at, c.updated_at,
		c.summary, c.summary_through_seq,
		r.id, r.conversation_id, r.bot_id, r.state, r.steps, r.failure, r.started_at, r.ended_at
		FROM conversations c
		LEFT JOIN runs r ON r.rowid = (SELECT MAX(rowid) FROM runs WHERE conversation_id = c.id)`

// Conversation is the conversation with id and its newest run, or ErrNotFound.
func (db *DB) Conversation(ctx context.Context, id string) (org.Conversation, error) {
	rows, err := queryAll(ctx, db.sql, "conversation "+id, scanConversation, conversationSelect+` WHERE c.id = ?`, id)
	if err != nil {
		return org.Conversation{}, err
	}
	if len(rows) == 0 {
		return org.Conversation{}, fmt.Errorf("conversation %s: %w", id, ErrNotFound)
	}
	return rows[0], nil
}

// PutSummary stores text as the summary of the conversation, standing for
// its messages up to throughSeq. It is ErrNotFound when there is no such
// conversation.
func (db *DB) PutSummary(ctx context.Context, conversationID, text string, throughSeq int64) error {
	return db.update(ctx, "conversation "+conversationID,
		`UPDATE conversations SET summary = ?, summary_through_seq = ? WHERE id = ?`, text, throughSeq, conversationID)
}

func scanConversation(row scanner) (org.Conversation, error) {
	var c org.Conversation
	var created, updated, started, ended sql.NullInt64
	var run struct {
		id, conv, bot sql.NullString
		state, steps  sql.NullInt64
		failure       sql.NullString
	}
	err := row.Scan(&c.ID, &c.Kind, &c.BotID, &c.Title, &created, &updated,
		&c.Summary, &c.SummaryThroughSeq,
		&run.id, &run.conv, &run.bot, &run.state, &run.steps, &run.failure, &started, &ended)
	c.CreatedAt, c.UpdatedAt = fromMillis(created), fromMillis(updated)
	if run.id.Valid {
		c.LatestRun = &org.Run{ID: run.id.String, ConversationID: run.conv.String, BotID: run.bot.String,
			State: org.RunState(run.state.Int64), Steps: int(run.steps.Int64), Failure: run.failure.String,
			StartedAt: fromMillis(started), EndedAt: fromMillis(ended)}
	}
	return c, err
}
