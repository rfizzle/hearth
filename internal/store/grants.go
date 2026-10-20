package store

import (
	"context"
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
// one transaction. The rows for the bot's own folder are kept as they are
// and any the request names for that folder are ignored, so an edit cannot
// take a bot out of its own folder. The grant to ask the owner is kept the
// same way: the owner never edits it, and a bot without it cannot ask. It is
// ErrNotFound when there is no such bot. The grants' BotID is not read; they
// are stored for botID.
// See docs/model.md#access.
func (db *DB) ReplaceGrants(ctx context.Context, botID string, grants []org.Grant) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace grants: %w", err)
	}
	defer tx.Rollback()

	var own string
	if err := tx.QueryRowContext(ctx, `SELECT folder_path FROM bots WHERE id = ?`, botID).Scan(&own); err != nil {
		return fmt.Errorf("replace grants: bot %s: %w", botID, notFound(err))
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM grants WHERE bot_id = ? AND folder <> ? AND capability <> ?`,
		botID, own, org.AskOwner); err != nil {
		return fmt.Errorf("replace grants: %w", err)
	}
	for _, g := range grants {
		if g.Folder == own || g.Capability == org.AskOwner {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO grants (bot_id, capability, mode, folder) VALUES (?, ?, ?, ?)`,
			botID, g.Capability, g.Mode, g.Folder); err != nil {
			return fmt.Errorf("replace grants: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("replace grants: %w", err)
	}
	return nil
}
