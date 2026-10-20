package sqlrow

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when the row asked for does not exist.
var ErrNotFound = errors.New("not found")

// NewID is a random 128-bit id in hex.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Millis is t as Unix milliseconds, and NULL for the zero time.
func Millis(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

// FromMillis is the inverse of Millis.
func FromMillis(n sql.NullInt64) time.Time {
	if !n.Valid {
		return time.Time{}
	}
	return time.UnixMilli(n.Int64).UTC()
}

// NullString stores the empty string as NULL, for a column that references
// another table.
func NullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// Scanner is a *sql.Row or *sql.Rows.
type Scanner interface{ Scan(...any) error }

// QueryAll runs query and reads every row with scan. what names the rows in
// an error.
func QueryAll[T any](ctx context.Context, db *sql.DB, what string, scan func(Scanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return out, nil
}

// Update runs an UPDATE of one row, and is ErrNotFound when it changed none.
// what names the row in an error.
func Update(ctx context.Context, db *sql.DB, what, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update %s: %w", what, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("update %s: %w", what, OrNotFound(err))
	}
	return nil
}

// DeleteByID removes the row of table with id, and is ErrNotFound when there
// is none. what names the row in an error.
func DeleteByID(ctx context.Context, db *sql.DB, what, table, id string) error {
	res, err := db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete %s %s: %w", what, id, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("delete %s %s: %w", what, id, OrNotFound(err))
	}
	return nil
}

// InTx runs fn in one transaction and commits it when fn succeeds. what
// names the operation in an error.
func InTx(ctx context.Context, db *sql.DB, what string, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	defer tx.Rollback()
	if err = fn(tx); err == nil {
		err = tx.Commit()
	}
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return nil
}

// NotFound turns sql.ErrNoRows into ErrNotFound.
func NotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// OrNotFound is err, or ErrNotFound when an update found no row to change.
func OrNotFound(err error) error {
	if err != nil {
		return err
	}
	return ErrNotFound
}
