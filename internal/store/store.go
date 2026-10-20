package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// ErrNotFound is returned when the row asked for does not exist.
var ErrNotFound = errors.New("not found")

// ErrFolderTaken is returned when a new bot's folder is another bot's,
// whatever the case.
var ErrFolderTaken = errors.New("another bot already has that folder")

// DB is an open store.
type DB struct {
	sql *sql.DB
	now func() time.Time
}

// Open opens the store at path, creating the file if it is missing, and
// brings its schema up to date.
//
// _txlock=immediate makes every transaction take the write lock when it
// begins: a deferred one that reads first and writes second is refused
// outright in WAL mode when another writer got there in between, where an
// immediate one waits out the busy timeout instead.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	// One connection: SQLite has one writer anyway, and the pragmas above are
	// per connection, so a pool would only add connections to configure.
	conn.SetMaxOpenConns(1)
	db := &DB{sql: conn, now: time.Now}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the store.
func (db *DB) Close() error {
	return db.sql.Close()
}

// clock is the time now, to the millisecond the store keeps.
func (db *DB) clock() time.Time {
	return time.UnixMilli(db.now().UnixMilli()).UTC()
}

// newID is a random 128-bit id in hex.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// millis is t as Unix milliseconds, and NULL for the zero time.
func millis(t time.Time) sql.NullInt64 {
	if t.IsZero() {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

// fromMillis is the inverse of millis.
func fromMillis(n sql.NullInt64) time.Time {
	if !n.Valid {
		return time.Time{}
	}
	return time.UnixMilli(n.Int64).UTC()
}

// nullString stores the empty string as NULL, for a column that references
// another table.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// scanner is a *sql.Row or *sql.Rows.
type scanner interface{ Scan(...any) error }

// queryAll runs query and reads every row with scan. what names the rows in
// an error.
func queryAll[T any](ctx context.Context, db *sql.DB, what string, scan func(scanner) (T, error), query string, args ...any) ([]T, error) {
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

// update runs an UPDATE of one row, and is ErrNotFound when it changed none.
// what names the row in an error.
func (db *DB) update(ctx context.Context, what, query string, args ...any) error {
	res, err := db.sql.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update %s: %w", what, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("update %s: %w", what, orNotFound(err))
	}
	return nil
}

// notFound turns sql.ErrNoRows into ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// orNotFound is err, or ErrNotFound when an update found no row to change.
func orNotFound(err error) error {
	if err != nil {
		return err
	}
	return ErrNotFound
}
