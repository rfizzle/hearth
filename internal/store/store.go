package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/rfizzle/hearth/internal/store/books"
	"github.com/rfizzle/hearth/internal/store/migrate"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// ErrNotFound is returned when the row asked for does not exist, by the
// store and by every repository it embeds.
var ErrNotFound = sqlrow.ErrNotFound

// ErrFolderTaken is returned when a new bot's folder is another bot's,
// whatever the case.
var ErrFolderTaken = errors.New("another bot already has that folder")

// DB is an open store. The repositories of the sub-packages are embedded,
// so their methods are the store's.
type DB struct {
	*books.Repo

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
	db.Repo = books.New(conn, db.clock)
	if err := migrate.Up(context.Background(), conn); err != nil {
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

// jsonField is a nullable JSON column and where its value goes.
type jsonField struct {
	raw  sql.NullString
	into any
	what string
}

// decodeFields reads each field that is not NULL. owner names the row in an
// error.
func decodeFields(owner string, fields ...jsonField) error {
	for _, f := range fields {
		if !f.raw.Valid {
			continue
		}
		if err := json.Unmarshal([]byte(f.raw.String), f.into); err != nil {
			return fmt.Errorf("%s %s: %w", owner, f.what, err)
		}
	}
	return nil
}
