package books

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

const noteFields = `id, bot_id, text, pinned, conversation_id, message_id, created_at`

const noteColumns = `SELECT ` + noteFields + ` FROM bot_notes`

// AddNote stores n as a note of n.BotID, minting its id and stamping it with
// the time when it has none, and returns it as stored. It is sqlrow.ErrNotFound
// when the bot does not exist.
func (repo *Repo) AddNote(ctx context.Context, n memory.Note) (memory.Note, error) {
	n.ID = sqlrow.NewID()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = repo.clock()
	}
	if err := insertNote(ctx, repo.sql, n); err != nil {
		return memory.Note{}, fmt.Errorf("add note: %w", err)
	}
	return n, nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// insertNote stores n, which is sqlrow.ErrNotFound when its bot does not exist.
func insertNote(ctx context.Context, e execer, n memory.Note) error {
	_, err := e.ExecContext(ctx, `INSERT INTO bot_notes (id, bot_id, text, pinned, conversation_id, message_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.BotID, n.Text, n.Pinned, n.ConversationID, n.MessageID, n.CreatedAt.UnixMilli())
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY") {
		return fmt.Errorf("bot %s: %w", n.BotID, sqlrow.ErrNotFound)
	}
	return err
}

// Notes is every note the bot keeps, oldest first.
func (repo *Repo) Notes(ctx context.Context, botID string) ([]memory.Note, error) {
	return sqlrow.QueryAll(ctx, repo.sql, "notes", scanNote, noteColumns+` WHERE bot_id = ? ORDER BY created_at, rowid`, botID)
}

// EditNote changes the text of the note with id and returns it. It is
// sqlrow.ErrNotFound when there is none.
func (repo *Repo) EditNote(ctx context.Context, id, text string) (memory.Note, error) {
	return repo.changeNote(ctx, id, "text", text)
}

// PinNote pins or unpins the note with id and returns it. A pinned note
// survives ReplaceNotes. It is sqlrow.ErrNotFound when there is none.
func (repo *Repo) PinNote(ctx context.Context, id string, pinned bool) (memory.Note, error) {
	return repo.changeNote(ctx, id, "pinned", pinned)
}

// changeNote sets one column of the note with id and returns the note.
func (repo *Repo) changeNote(ctx context.Context, id, column string, value any) (memory.Note, error) {
	n, err := scanNote(repo.sql.QueryRowContext(ctx, "UPDATE bot_notes SET "+column+" = ? WHERE id = ? RETURNING "+noteFields,
		value, id))
	if err != nil {
		return memory.Note{}, fmt.Errorf("update note %s: %w", id, sqlrow.NotFound(err))
	}
	return n, nil
}

// DeleteNote removes the note with id. It is sqlrow.ErrNotFound when there is none.
func (repo *Repo) DeleteNote(ctx context.Context, id string) error {
	return sqlrow.DeleteByID(ctx, repo.sql, "note", "bot_notes", id)
}

// ReplaceNotes swaps every unpinned note of the bot for the notes given, in
// one transaction: pinned notes stay as they are, and a failure leaves all
// of them as they were. The new notes are stored unpinned with fresh ids and
// the bot's id; the time of one that has none is now.
func (repo *Repo) ReplaceNotes(ctx context.Context, botID string, unpinned []memory.Note) error {
	now := repo.clock()
	return sqlrow.InTx(ctx, repo.sql, "replace notes", func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM bot_notes WHERE bot_id = ? AND pinned = 0`, botID)
		for i := 0; err == nil && i < len(unpinned); i++ {
			n := unpinned[i]
			n.ID, n.BotID, n.Pinned = sqlrow.NewID(), botID, false
			if n.CreatedAt.IsZero() {
				n.CreatedAt = now
			}
			err = insertNote(ctx, tx, n)
		}
		return err
	})
}

func scanNote(row sqlrow.Scanner) (memory.Note, error) {
	var n memory.Note
	var at int64
	if err := row.Scan(&n.ID, &n.BotID, &n.Text, &n.Pinned, &n.ConversationID, &n.MessageID, &at); err != nil {
		return memory.Note{}, err
	}
	n.CreatedAt = time.UnixMilli(at).UTC()
	return n, nil
}
