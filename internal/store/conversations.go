package store

import (
	"context"
	"database/sql"

	"github.com/rfizzle/hearth/internal/org"
)

// Conversations is every conversation the bot with botID is in, the most
// recently active first.
func (db *DB) Conversations(ctx context.Context, botID string) ([]org.Conversation, error) {
	return queryAll(ctx, db.sql, "conversations", scanConversation, `SELECT id, kind, COALESCE(bot_id, ''), title, created_at, updated_at
		FROM conversations WHERE bot_id = ? ORDER BY updated_at DESC, rowid DESC`, botID)
}

func scanConversation(row scanner) (org.Conversation, error) {
	var c org.Conversation
	var created, updated sql.NullInt64
	err := row.Scan(&c.ID, &c.Kind, &c.BotID, &c.Title, &created, &updated)
	c.CreatedAt, c.UpdatedAt = fromMillis(created), fromMillis(updated)
	return c, err
}
