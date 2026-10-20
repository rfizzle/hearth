package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/inbox"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// AddAllowance lets a.ConversationID use a.Capability in a.Folder without
// asking. Adding one that is already there changes nothing. It is ErrNotFound
// when the conversation does not exist.
func (db *DB) AddAllowance(ctx context.Context, a inbox.Allowance) error {
	var n int
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversations WHERE id = ?`, a.ConversationID).Scan(&n); err != nil {
		return fmt.Errorf("add allowance: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("add allowance: conversation %s: %w", a.ConversationID, ErrNotFound)
	}
	if _, err := db.sql.ExecContext(ctx, `INSERT OR IGNORE INTO allowances (conversation_id, capability, folder)
		VALUES (?, ?, ?)`, a.ConversationID, a.Capability, a.Folder); err != nil {
		return fmt.Errorf("add allowance: %w", err)
	}
	return nil
}

// Allowances is every allowance the conversation holds, in the order they
// were added. A conversation with none, or no such conversation, has an
// empty list.
func (db *DB) Allowances(ctx context.Context, conversationID string) ([]inbox.Allowance, error) {
	scan := func(row sqlrow.Scanner) (a inbox.Allowance, err error) {
		var c string
		err = row.Scan(&a.ConversationID, &c, &a.Folder)
		a.Capability = org.Capability(c)
		return a, err
	}
	return sqlrow.QueryAll(ctx, db.sql, "allowances", scan,
		`SELECT conversation_id, capability, folder FROM allowances WHERE conversation_id = ? ORDER BY rowid`, conversationID)
}
