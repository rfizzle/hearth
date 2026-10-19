package store

import (
	"context"
	"fmt"
)

// migrations is append-only: a new step goes at the end, and a released step
// is never inserted before, removed or edited, because the recorded version
// is a count and a store already past a position records whatever is put
// there as done without running it. The digest test holds the list to that.
var migrations = []string{
	`CREATE TABLE org (
		id             INTEGER PRIMARY KEY CHECK (id = 1),
		name           TEXT NOT NULL,
		workspace_path TEXT NOT NULL
	);

	CREATE TABLE bots (
		id              TEXT PRIMARY KEY,
		name            TEXT NOT NULL,
		role            TEXT NOT NULL DEFAULT '',
		job_description TEXT NOT NULL DEFAULT '',
		tier            INTEGER NOT NULL,
		status          INTEGER NOT NULL,
		avatar          TEXT NOT NULL DEFAULT '',
		folder_path     TEXT NOT NULL,
		created_at      INTEGER NOT NULL
	);

	CREATE TABLE grants (
		id         INTEGER PRIMARY KEY,
		bot_id     TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
		capability TEXT NOT NULL,
		mode       INTEGER NOT NULL,
		folder     TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX grants_bot ON grants(bot_id);

	CREATE TABLE conversations (
		id         TEXT PRIMARY KEY,
		kind       INTEGER NOT NULL,
		bot_id     TEXT REFERENCES bots(id) ON DELETE CASCADE,
		title      TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);
	CREATE INDEX conversations_bot ON conversations(bot_id);

	CREATE TABLE runs (
		id              TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		bot_id          TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
		state           INTEGER NOT NULL,
		steps           INTEGER NOT NULL DEFAULT 0,
		failure         TEXT NOT NULL DEFAULT '',
		started_at      INTEGER,
		ended_at        INTEGER
	);
	CREATE INDEX runs_state ON runs(state);

	CREATE TABLE messages (
		id              TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		seq             INTEGER NOT NULL,
		author_bot_id   TEXT NOT NULL DEFAULT '',
		kind            INTEGER NOT NULL,
		content         TEXT NOT NULL DEFAULT '',
		tool_calls      TEXT,
		tool_call_id    TEXT NOT NULL DEFAULT '',
		files           TEXT,
		run_id          TEXT REFERENCES runs(id) ON DELETE SET NULL,
		streaming       INTEGER NOT NULL DEFAULT 0,
		created_at      INTEGER NOT NULL,
		UNIQUE(conversation_id, seq)
	);
	CREATE INDEX messages_run ON messages(run_id);`,
}

// migrate brings the store up to the current schema, one step per
// transaction. Each step runs under BEGIN IMMEDIATE with the version re-read
// inside the lock, so two processes opening the same file at once cannot
// both apply a step: the second waits out the busy timeout, finds the step
// recorded and moves on. The version table is created under the same lock,
// because as a bare statement it would read before it writes, and a WAL
// writer whose read has gone stale is refused rather than made to wait.
func (db *DB) migrate() error {
	ctx := context.Background()
	// One connection for the whole run: BEGIN IMMEDIATE is a statement, and
	// the step and its COMMIT have to run on the connection that began it.
	conn, err := db.sql.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		return fmt.Errorf("create schema_version: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}

	for {
		if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
			return fmt.Errorf("begin migration: %w", err)
		}
		var current int
		if err := conn.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
			return fmt.Errorf("read schema version: %w", err)
		}
		if current >= len(migrations) {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
			return nil
		}
		if _, err := conn.ExecContext(ctx, migrations[current]); err != nil {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
			return fmt.Errorf("migration %d: %w", current+1, err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, current+1); err != nil {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
			return fmt.Errorf("record migration %d: %w", current+1, err)
		}
		if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
			return fmt.Errorf("commit migration %d: %w", current+1, err)
		}
	}
}
