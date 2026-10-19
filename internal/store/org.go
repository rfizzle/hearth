package store

import (
	"context"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// Org is the org, or ErrNotFound before the owner has named it.
func (db *DB) Org(ctx context.Context) (org.Org, error) {
	var o org.Org
	err := db.sql.QueryRowContext(ctx, `SELECT name, workspace_path FROM org WHERE id = 1`).
		Scan(&o.Name, &o.WorkspacePath)
	if err != nil {
		return org.Org{}, fmt.Errorf("org: %w", notFound(err))
	}
	return o, nil
}

// PutOrg saves the org, creating its one row or replacing it.
func (db *DB) PutOrg(ctx context.Context, o org.Org) error {
	_, err := db.sql.ExecContext(ctx, `INSERT INTO org (id, name, workspace_path) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, workspace_path = excluded.workspace_path`,
		o.Name, o.WorkspacePath)
	if err != nil {
		return fmt.Errorf("put org: %w", err)
	}
	return nil
}
