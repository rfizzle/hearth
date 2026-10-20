package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// BackfillAlwaysOn gives every bot that holds no grant for capability, one
// every bot starts with (asking the owner, remembering), the grant a new bot
// starts with, allowed, and returns how many it gave. A bot hired before the
// capability existed has none, and without it the bot cannot use it. A bot
// that has the grant, whatever its mode, is left alone.
// See docs/model.md#access.
func (db *DB) BackfillAlwaysOn(ctx context.Context, capability org.Capability) (int64, error) {
	res, err := db.sql.ExecContext(ctx, `INSERT INTO grants (bot_id, capability, mode, folder)
		SELECT id, ?, ?, '' FROM bots
		WHERE NOT EXISTS (SELECT 1 FROM grants WHERE grants.bot_id = bots.id AND grants.capability = ?)`,
		capability, org.ModeAllowed, capability)
	if err != nil {
		return 0, fmt.Errorf("backfill %s: %w", capability, err)
	}
	return res.RowsAffected()
}
