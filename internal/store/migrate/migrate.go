// Package migrate brings the store's SQLite file up to the current schema.
//
// Responsibility: the ordered list of schema steps and the runner that
// applies the ones a file has not seen.
//
// Invariants the types do not hold:
//   - The list is append-only. A released step is never edited, removed or
//     moved; a change is a new step at the end, and its digest is added to
//     the released list in the test.
//   - Each step runs in its own transaction, under the write lock.
//
// It is handed an open database and imports no driver.
package migrate

import (
	"context"
	"database/sql"
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

	`CREATE TABLE inbox_items (
		id              TEXT PRIMARY KEY,
		kind            INTEGER NOT NULL,
		state           INTEGER NOT NULL,
		bot_id          TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
		conversation_id TEXT REFERENCES conversations(id) ON DELETE CASCADE,
		run_id          TEXT REFERENCES runs(id) ON DELETE SET NULL,
		message_id      TEXT REFERENCES messages(id) ON DELETE SET NULL,
		title           TEXT NOT NULL DEFAULT '',
		effect          TEXT NOT NULL DEFAULT '',
		call            TEXT,
		question        TEXT NOT NULL DEFAULT '',
		suggested       TEXT,
		widget_spec     TEXT,
		widget_answer   TEXT,
		created_at      INTEGER NOT NULL,
		closed_at       INTEGER,
		decision        INTEGER NOT NULL DEFAULT 0,
		answer          TEXT NOT NULL DEFAULT '',
		outcome         TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX inbox_items_state ON inbox_items(state);
	CREATE INDEX inbox_items_conversation ON inbox_items(conversation_id);
	CREATE UNIQUE INDEX inbox_items_one_open ON inbox_items(conversation_id) WHERE state = 1;

	CREATE TABLE allowances (
		conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
		capability      TEXT NOT NULL,
		folder          TEXT NOT NULL DEFAULT '',
		UNIQUE(conversation_id, capability, folder)
	);

	ALTER TABLE messages ADD COLUMN widget_spec TEXT;
	ALTER TABLE messages ADD COLUMN widget_answer TEXT;`,

	`CREATE TABLE bot_notes (
		id              TEXT PRIMARY KEY,
		bot_id          TEXT NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
		text            TEXT NOT NULL,
		pinned          INTEGER NOT NULL DEFAULT 0,
		conversation_id TEXT NOT NULL DEFAULT '',
		message_id      TEXT NOT NULL DEFAULT '',
		created_at      INTEGER NOT NULL
	);
	CREATE INDEX bot_notes_bot ON bot_notes(bot_id);

	CREATE TABLE handbook_pages (
		id           TEXT PRIMARY KEY,
		title        TEXT NOT NULL,
		markdown     TEXT NOT NULL DEFAULT '',
		scope_bot_id TEXT REFERENCES bots(id) ON DELETE CASCADE,
		updated_at   INTEGER NOT NULL
	);

	CREATE TABLE usage (
		id              INTEGER PRIMARY KEY,
		run_id          TEXT NOT NULL DEFAULT '',
		bot_id          TEXT NOT NULL DEFAULT '',
		conversation_id TEXT NOT NULL DEFAULT '',
		task_id         TEXT,
		month           TEXT NOT NULL,
		provider        TEXT NOT NULL DEFAULT '',
		model           TEXT NOT NULL DEFAULT '',
		prompt          INTEGER NOT NULL DEFAULT 0,
		completion      INTEGER NOT NULL DEFAULT 0,
		cached          INTEGER NOT NULL DEFAULT 0,
		created         INTEGER NOT NULL DEFAULT 0,
		plan            INTEGER NOT NULL DEFAULT 0,
		cost_micros     INTEGER NOT NULL DEFAULT 0,
		priced          INTEGER NOT NULL DEFAULT 0,
		at              INTEGER NOT NULL
	);
	CREATE INDEX usage_month_bot ON usage(month, bot_id);
	CREATE INDEX usage_run ON usage(run_id);

	CREATE TABLE budgets (
		subject        TEXT PRIMARY KEY,
		cents          INTEGER NOT NULL DEFAULT 0,
		paused_month   TEXT NOT NULL DEFAULT '',
		resumed_month  TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE tiers (
		tier     INTEGER PRIMARY KEY,
		provider TEXT NOT NULL,
		profile  TEXT NOT NULL DEFAULT '',
		model    TEXT NOT NULL
	);

	ALTER TABLE conversations ADD COLUMN summary TEXT NOT NULL DEFAULT '';
	ALTER TABLE conversations ADD COLUMN summary_through_seq INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE bots ADD COLUMN model_override TEXT NOT NULL DEFAULT '';
	ALTER TABLE messages ADD COLUMN attachments TEXT;`,

	// A bot works on a team (1) or for the whole org (2); every bot hired
	// before scopes existed was a team member.
	`ALTER TABLE bots ADD COLUMN scope INTEGER NOT NULL DEFAULT 1;`,

	// The introduction a bot was hired with, as JSON; empty for a bot hired
	// before introductions existed, which the store reads as an assembled one.
	`ALTER TABLE bots ADD COLUMN introduction TEXT NOT NULL DEFAULT '';`,
}

// Count is the number of steps, the version a current file records.
func Count() int { return len(migrations) }

// Up brings db up to the current schema, one step per
// transaction. Each step runs under BEGIN IMMEDIATE with the version re-read
// inside the lock, so two processes opening the same file at once cannot
// both apply a step: the second waits out the busy timeout, finds the step
// recorded and moves on. The version table is created under the same lock,
// because as a bare statement it would read before it writes, and a WAL
// writer whose read has gone stale is refused rather than made to wait.
func Up(ctx context.Context, db *sql.DB) error {
	// One connection for the whole run: BEGIN IMMEDIATE is a statement, and
	// the step and its COMMIT have to run on the connection that began it.
	conn, err := db.Conn(ctx)
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
