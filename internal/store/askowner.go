package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// BackfillAskOwner gives every bot that holds no grant to ask the owner the
// one a new bot starts with, allowed, and returns how many it gave. A bot
// hired before asking existed has none, and without it the bot cannot ask a
// question. A bot that has the grant, whatever its mode, is left alone.
// See docs/model.md#access.
func (db *DB) BackfillAskOwner(ctx context.Context) (int64, error) {
	res, err := db.sql.ExecContext(ctx, `INSERT INTO grants (bot_id, capability, mode, folder)
		SELECT id, ?, ?, '' FROM bots
		WHERE NOT EXISTS (SELECT 1 FROM grants WHERE grants.bot_id = bots.id AND grants.capability = ?)`,
		org.AskOwner, org.ModeAllowed, org.AskOwner)
	if err != nil {
		return 0, fmt.Errorf("backfill ask owner: %w", err)
	}
	return res.RowsAffected()
}
