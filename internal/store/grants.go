package store

import (
	"context"
	"database/sql"
	"fmt"

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

// ReplaceGrants makes grants the whole of what the bot with botID holds, in
// one transaction. The rows for the bot's own folder are never removed: the
// request sets their modes, and a capability it leaves out keeps its row as
// it was, so an edit cannot take a bot out of its own folder. The grants to ask the owner and to remember are kept the
// same way: the owner never edits them, and a bot without them cannot. It is
// ErrNotFound when there is no such bot. The grants' BotID is not read; they
// are stored for botID.
// See docs/model.md#access.
func (db *DB) ReplaceGrants(ctx context.Context, botID string, grants []org.Grant) error {
	return db.inTx(ctx, "replace grants", func(tx *sql.Tx) error {
		var own string
		if err := tx.QueryRowContext(ctx, `SELECT folder_path FROM bots WHERE id = ?`, botID).Scan(&own); err != nil {
			return fmt.Errorf("bot %s: %w", botID, notFound(err))
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM grants WHERE bot_id = ? AND folder <> ? AND capability NOT IN (?, ?)`,
			botID, own, org.AskOwner, org.Remember); err != nil {
			return err
		}
		for _, g := range grants {
			if g.Capability == org.AskOwner {
				continue
			}
			q, args := `INSERT INTO grants (bot_id, capability, mode, folder) VALUES (?, ?, ?, ?)`,
				[]any{botID, g.Capability, g.Mode, g.Folder}
			if g.Folder == own {
				q, args = `UPDATE grants SET mode = ? WHERE bot_id = ? AND capability = ? AND folder = ?`,
					[]any{g.Mode, botID, g.Capability, own}
			}
			if _, err := tx.ExecContext(ctx, q, args...); err != nil {
				return err
			}
		}
		return nil
	})
}
