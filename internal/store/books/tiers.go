package books

import (
	"context"
	"database/sql"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// TierChoice is the model a tier maps to: the provider, the added service
// (Profile, empty for a built-in provider) and the model name.
type TierChoice struct {
	Tier     org.Tier
	Provider string
	Profile  string
	Model    string
}

// Tiers is the mapping the owner has chosen, by tier. A tier with no row
// follows the defaults.
func (repo *Repo) Tiers(ctx context.Context) ([]TierChoice, error) {
	return sqlrow.QueryAll(ctx, repo.sql, "tiers", func(row sqlrow.Scanner) (c TierChoice, err error) {
		err = row.Scan(&c.Tier, &c.Provider, &c.Profile, &c.Model)
		return c, err
	}, `SELECT tier, provider, profile, model FROM tiers ORDER BY tier`)
}

// PutTiers replaces the mapping with choices in one transaction.
func (repo *Repo) PutTiers(ctx context.Context, choices []TierChoice) error {
	return sqlrow.InTx(ctx, repo.sql, "put tiers", func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM tiers`)
		for i := 0; err == nil && i < len(choices); i++ {
			c := choices[i]
			_, err = tx.ExecContext(ctx, `INSERT INTO tiers (tier, provider, profile, model) VALUES (?, ?, ?, ?)`,
				c.Tier, c.Provider, c.Profile, c.Model)
		}
		return err
	})
}
