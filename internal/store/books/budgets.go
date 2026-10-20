package books

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org/usage"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// Budgets is every budget set, by subject.
func (repo *Repo) Budgets(ctx context.Context) ([]usage.Budget, error) {
	return sqlrow.QueryAll(ctx, repo.sql, "budgets", func(row sqlrow.Scanner) (b usage.Budget, err error) {
		err = row.Scan(&b.Subject, &b.Cents, &b.PausedMonth, &b.ResumedMonth)
		return b, err
	}, `SELECT subject, cents, paused_month, resumed_month FROM budgets ORDER BY subject`)
}

// PutBudget sets the limit in cents of b.Subject, adding the budget when
// the subject has none. A pause or resume the subject already has is kept.
func (repo *Repo) PutBudget(ctx context.Context, b usage.Budget) error {
	if _, err := repo.sql.ExecContext(ctx, `INSERT INTO budgets (subject, cents) VALUES (?, ?)
		ON CONFLICT(subject) DO UPDATE SET cents = excluded.cents`, b.Subject, b.Cents); err != nil {
		return fmt.Errorf("put budget: %w", err)
	}
	return nil
}

// SetPaused records the months the subject was paused for and resumed in;
// "" clears one. A subject with no budget gets a row with a limit of 0.
func (repo *Repo) SetPaused(ctx context.Context, subject, pausedMonth, resumedMonth string) error {
	if _, err := repo.sql.ExecContext(ctx, `INSERT INTO budgets (subject, paused_month, resumed_month) VALUES (?, ?, ?)
		ON CONFLICT(subject) DO UPDATE SET paused_month = excluded.paused_month,
		resumed_month = excluded.resumed_month`, subject, pausedMonth, resumedMonth); err != nil {
		return fmt.Errorf("set paused: %w", err)
	}
	return nil
}
