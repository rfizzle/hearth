package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org/usage"
)

// Budgets is every budget set, by subject.
func (db *DB) Budgets(ctx context.Context) ([]usage.Budget, error) {
	return queryAll(ctx, db.sql, "budgets", func(row scanner) (b usage.Budget, err error) {
		err = row.Scan(&b.Subject, &b.Cents, &b.PausedMonth, &b.ResumedMonth)
		return b, err
	}, `SELECT subject, cents, paused_month, resumed_month FROM budgets ORDER BY subject`)
}

// PutBudget sets the limit in cents of b.Subject, adding the budget when
// the subject has none. A pause or resume the subject already has is kept.
func (db *DB) PutBudget(ctx context.Context, b usage.Budget) error {
	if _, err := db.sql.ExecContext(ctx, `INSERT INTO budgets (subject, cents) VALUES (?, ?)
		ON CONFLICT(subject) DO UPDATE SET cents = excluded.cents`, b.Subject, b.Cents); err != nil {
		return fmt.Errorf("put budget: %w", err)
	}
	return nil
}

// SetPaused records the months the subject was paused for and resumed in;
// "" clears one. A subject with no budget gets a row with a limit of 0.
func (db *DB) SetPaused(ctx context.Context, subject, pausedMonth, resumedMonth string) error {
	if _, err := db.sql.ExecContext(ctx, `INSERT INTO budgets (subject, paused_month, resumed_month) VALUES (?, ?, ?)
		ON CONFLICT(subject) DO UPDATE SET paused_month = excluded.paused_month,
		resumed_month = excluded.resumed_month`, subject, pausedMonth, resumedMonth); err != nil {
		return fmt.Errorf("set paused: %w", err)
	}
	return nil
}
