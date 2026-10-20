package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// Org is the org, or ErrNotFound before the owner has named it.
func (db *DB) Org(ctx context.Context) (org.Org, error) {
	var o org.Org
	err := db.sql.QueryRowContext(ctx, `SELECT name, workspace_path FROM org WHERE id = 1`).
		Scan(&o.Name, &o.WorkspacePath)
	if err != nil {
		return org.Org{}, fmt.Errorf("org: %w", sqlrow.NotFound(err))
	}
	return o, nil
}

// FindOrg is the org, and false with no error before the owner has named it.
func (db *DB) FindOrg(ctx context.Context) (org.Org, bool, error) {
	o, err := db.Org(ctx)
	if errors.Is(err, ErrNotFound) {
		return org.Org{}, false, nil
	}
	return o, err == nil, err
}

// PutOrg saves the org, creating its one row or replacing it.
func (db *DB) PutOrg(ctx context.Context, o org.Org) error {
	if err := putOrg(ctx, db.sql, o); err != nil {
		return fmt.Errorf("put org: %w", err)
	}
	return nil
}

// execer is a *sql.DB or a *sql.Tx.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func putOrg(ctx context.Context, e execer, o org.Org) error {
	_, err := e.ExecContext(ctx, `INSERT INTO org (id, name, workspace_path) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, workspace_path = excluded.workspace_path`,
		o.Name, o.WorkspacePath)
	return err
}
