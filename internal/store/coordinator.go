package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// PutOrgWithCoordinator saves the org as PutOrg does and, in the same
// transaction, hires coordinator with its grants and its DM as an org-level
// bot when the org has none yet. It returns the bot and true when it hired
// it; when an org-level bot was already there it hires nobody and returns
// false, so saving the org again never makes a second Chief of Staff. A team
// bot already in the folder the coordinator would take leaves the org saved
// without one rather than refusing the save.
// See docs/model.md#people
func (db *DB) PutOrgWithCoordinator(ctx context.Context, o org.Org, coordinator org.Bot, grants []org.Grant, dm org.Conversation) (org.Bot, bool, error) {
	var hired org.Bot
	err := sqlrow.InTx(ctx, db.sql, "put org", func(tx *sql.Tx) error {
		if err := putOrg(ctx, tx, o); err != nil {
			return err
		}
		var have int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM bots WHERE scope = ?`, org.ScopeOrg).Scan(&have); err != nil {
			return err
		}
		if have > 0 {
			return nil
		}
		coordinator.Scope = org.ScopeOrg
		stored, err := db.insertBot(ctx, tx, coordinator, grants, dm)
		if errors.Is(err, ErrFolderTaken) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("hire %s: %w", coordinator.Name, err)
		}
		hired = stored
		return nil
	})
	if err != nil || hired.ID == "" {
		return org.Bot{}, false, err
	}
	return hired, true, nil
}

// Coordinator is the org-level bot that answers for the whole org, the
// Chief of Staff hired at onboarding: the oldest org-level bot, or
// ErrNotFound when there is none.
// See docs/model.md#5-the-chief-of-staffs-routing-policy
func (db *DB) Coordinator(ctx context.Context) (org.Bot, error) {
	b, err := scanBot(db.sql.QueryRowContext(ctx, botColumns+` WHERE b.scope = ? ORDER BY b.created_at, b.rowid LIMIT 1`,
		org.ConversationDM, org.ScopeOrg))
	if err != nil {
		return org.Bot{}, fmt.Errorf("coordinator: %w", sqlrow.NotFound(err))
	}
	return b, nil
}
