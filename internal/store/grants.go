package store

import (
	"context"

	"github.com/rfizzle/hearth/internal/org"
)

// GrantsFor is every grant the bot with botID holds, in the order they were
// given. A bot with none, or no such bot, has an empty list.
func (db *DB) GrantsFor(ctx context.Context, botID string) ([]org.Grant, error) {
	scan := func(row scanner) (g org.Grant, err error) {
		err = row.Scan(&g.BotID, &g.Capability, &g.Mode, &g.Folder)
		return g, err
	}
	return queryAll(ctx, db.sql, "grants", scan,
		`SELECT bot_id, capability, mode, folder FROM grants WHERE bot_id = ? ORDER BY id`, botID)
}
