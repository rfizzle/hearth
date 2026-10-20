package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// CreateRun stores a new run, minting its id, and returns it as stored.
func (db *DB) CreateRun(ctx context.Context, r org.Run) (org.Run, error) {
	r.ID = newID()
	_, err := db.sql.ExecContext(ctx, `INSERT INTO runs
		(id, conversation_id, bot_id, state, steps, failure, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ConversationID, r.BotID, r.State, r.Steps, r.Failure, millis(r.StartedAt), millis(r.EndedAt))
	if err != nil {
		return org.Run{}, fmt.Errorf("create run: %w", err)
	}
	return r, nil
}

// UpdateRun replaces a stored run's state, steps, failure and times. It is
// ErrNotFound when no run has r.ID.
func (db *DB) UpdateRun(ctx context.Context, r org.Run) error {
	return db.update(ctx, "run "+r.ID, `UPDATE runs
		SET state = ?, steps = ?, failure = ?, started_at = ?, ended_at = ? WHERE id = ?`,
		r.State, r.Steps, r.Failure, millis(r.StartedAt), millis(r.EndedAt), r.ID)
}

// Run is the run with id, or ErrNotFound.
func (db *DB) Run(ctx context.Context, id string) (org.Run, error) {
	var r org.Run
	var started, ended sql.NullInt64
	err := db.sql.QueryRowContext(ctx, `SELECT id, conversation_id, bot_id, state, steps, failure, started_at, ended_at
		FROM runs WHERE id = ?`, id).
		Scan(&r.ID, &r.ConversationID, &r.BotID, &r.State, &r.Steps, &r.Failure, &started, &ended)
	if err != nil {
		return org.Run{}, fmt.Errorf("run %s: %w", id, notFound(err))
	}
	r.StartedAt, r.EndedAt = fromMillis(started), fromMillis(ended)
	return r, nil
}

// LatestRun is the newest run in the conversation convID, in whatever state,
// and false when it has had none. Newest is the order the runs were created
// in, which is the order they started in.
func (db *DB) LatestRun(ctx context.Context, convID string) (org.Run, bool, error) {
	rows, err := queryAll(ctx, db.sql, "latest run of "+convID, scanRun, `SELECT `+runColumns+`
		FROM runs WHERE conversation_id = ? ORDER BY rowid DESC LIMIT 1`, convID)
	if err != nil || len(rows) == 0 {
		return org.Run{}, false, err
	}
	return rows[0], true, nil
}

// runColumns is what scanRun reads.
const runColumns = `id, conversation_id, bot_id, state, steps, failure, started_at, ended_at`

func scanRun(row scanner) (org.Run, error) {
	var r org.Run
	var started, ended sql.NullInt64
	err := row.Scan(&r.ID, &r.ConversationID, &r.BotID, &r.State, &r.Steps, &r.Failure, &started, &ended)
	r.StartedAt, r.EndedAt = fromMillis(started), fromMillis(ended)
	return r, err
}

// FailWorkingRuns ends every run still queued or working as failed with
// reason, and returns how many it ended. It is for startup, before any run
// begins: a run still in either state then was cut off by the core stopping,
// and would otherwise show its bot working forever.
func (db *DB) FailWorkingRuns(ctx context.Context, reason string) (int64, error) {
	res, err := db.sql.ExecContext(ctx, `UPDATE runs SET state = ?, failure = ?, ended_at = ? WHERE state IN (?, ?)`,
		org.RunFailed, reason, db.clock().UnixMilli(), org.RunQueued, org.RunWorking)
	if err != nil {
		return 0, fmt.Errorf("fail working runs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("fail working runs: %w", err)
	}
	return n, nil
}
