package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/project"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// CreateProject stores p, minting its id, and returns it as stored. An
// active project is stored with its conversation (kind and project id
// filled in) in one transaction, so none exists without the other; a
// proposed one has no folder or conversation yet. A folder another bot or
// project has is ErrFolderTaken.
// See docs/model.md#work
func (db *DB) CreateProject(ctx context.Context, p project.Project, conv org.Conversation) (project.Project, error) {
	err := db.inProjectTx(ctx, "create project", func(tx *sql.Tx, now time.Time) error {
		p.ID = sqlrow.NewID()
		p.CreatedAt, p.UpdatedAt = now, now
		if p.State == project.Active {
			p.ActivatedAt = now
		}
		if p.FolderPath != "" {
			if err := checkFolderFree(ctx, tx, p.FolderPath); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO projects
			(id, name, goal, context, done_looks_like, done_when, state, target_date, folder_path,
			 proposed_by_bot_id, proposed_in_conversation_id, created_at, activated_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			p.ID, p.Name, p.Brief.Goal, p.Brief.Context, p.Brief.DoneLooksLike, p.Brief.DoneWhen, p.State,
			p.TargetDate, p.FolderPath, sqlrow.NullString(p.ProposedByBotID), sqlrow.NullString(p.ProposedInConversationID),
			now.UnixMilli(), sqlrow.Millis(p.ActivatedAt), now.UnixMilli()); err != nil {
			return err
		}
		if p.State != project.Active {
			return nil
		}
		var err error
		p.ConversationID, err = db.insertProjectConversation(ctx, tx, p, conv)
		return err
	})
	if err != nil {
		return project.Project{}, err
	}
	return p, nil
}

// ActivateProject makes the proposal with p.ID active with p's name, brief,
// target date and folder, and stores conv as its conversation, in one
// transaction. It is ErrNotFound when there is no such proposal.
func (db *DB) ActivateProject(ctx context.Context, p project.Project, conv org.Conversation) (project.Project, error) {
	err := db.inProjectTx(ctx, "activate project", func(tx *sql.Tx, now time.Time) error {
		if err := checkFolderFree(ctx, tx, p.FolderPath); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE projects
			SET name = ?, goal = ?, context = ?, done_looks_like = ?, done_when = ?, state = ?,
			    target_date = ?, folder_path = ?, activated_at = ?, updated_at = ?
			WHERE id = ? AND state = ?`,
			p.Name, p.Brief.Goal, p.Brief.Context, p.Brief.DoneLooksLike, p.Brief.DoneWhen, project.Active,
			p.TargetDate, p.FolderPath, now.UnixMilli(), now.UnixMilli(), p.ID, project.Proposed)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil || n == 0 {
			return fmt.Errorf("proposal %s: %w", p.ID, ErrNotFound)
		}
		p.ConversationID, err = db.insertProjectConversation(ctx, tx, p, conv)
		return err
	})
	if err != nil {
		return project.Project{}, err
	}
	return db.Project(ctx, p.ID)
}

// inProjectTx runs fn in one transaction with the time its rows are stamped.
func (db *DB) inProjectTx(ctx context.Context, what string, fn func(*sql.Tx, time.Time) error) error {
	now := db.clock()
	return sqlrow.InTx(ctx, db.sql, what, func(tx *sql.Tx) error { return fn(tx, now) })
}

// insertProjectConversation stores conv as the conversation of p inside tx
// and returns its id.
func (db *DB) insertProjectConversation(ctx context.Context, tx *sql.Tx, p project.Project, conv org.Conversation) (string, error) {
	now := db.clock().UnixMilli()
	id := sqlrow.NewID()
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversations (id, kind, project_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, id, org.ConversationProject, p.ID, conv.Title, now, now); err != nil {
		return "", fmt.Errorf("project conversation: %w", err)
	}
	return id, nil
}

// UpdateProject sets what the owner may edit on the project with p.ID: its
// name, brief and target date. Its folder never changes, so a rename leaves
// the files where they are. It is ErrNotFound when there is no such project.
func (db *DB) UpdateProject(ctx context.Context, p project.Project) error {
	b := p.Brief
	edit := []any{p.Name, b.Goal, b.Context, b.DoneLooksLike, b.DoneWhen, p.TargetDate, db.clock().UnixMilli(), p.ID}
	return sqlrow.Update(ctx, db.sql, "project "+p.ID, `UPDATE projects
		SET name = ?, goal = ?, context = ?, done_looks_like = ?, done_when = ?, target_date = ?, updated_at = ?
		WHERE id = ?`, edit...)
}

// SetProjectState moves the project with id to s, stamping done_at or
// archived_at with at for Done or Archived. Nothing is deleted: an archived
// project keeps its row, its folder and its conversation. It is ErrNotFound
// when there is no such project.
func (db *DB) SetProjectState(ctx context.Context, id string, s project.State, at time.Time) error {
	column := "updated_at"
	switch s {
	case project.Done:
		column = "done_at"
	case project.Archived:
		column = "archived_at"
	}
	return sqlrow.Update(ctx, db.sql, "project "+id,
		`UPDATE projects SET state = ?, `+column+` = ?, updated_at = ? WHERE id = ?`,
		s, at.UnixMilli(), db.clock().UnixMilli(), id)
}

// EditProject applies the owner's edits to the project with id as
// project.Edit allows and returns it as stored. The folder stays.
func (db *DB) EditProject(ctx context.Context, id, name string, brief project.Brief, targetDate string) (project.Project, error) {
	p, err := db.Project(ctx, id)
	if err == nil {
		p, err = project.Edit(p, name, brief, targetDate)
	}
	if err == nil {
		err = db.UpdateProject(ctx, p)
	}
	if err != nil {
		return project.Project{}, err
	}
	return db.Project(ctx, id)
}

// MoveProject takes the project with id to state next, stamped at, if its
// state allows it (project.State.CanBecome), and returns it as stored.
func (db *DB) MoveProject(ctx context.Context, id string, next project.State, at time.Time) (project.Project, error) {
	p, err := db.Project(ctx, id)
	if err == nil {
		err = p.State.CanBecome(next)
	}
	if err == nil {
		err = db.SetProjectState(ctx, id, next, at)
	}
	if err != nil {
		return project.Project{}, err
	}
	return db.Project(ctx, id)
}

// projectSelect reads a project with the id of its conversation, which lives
// on the conversation rather than the project; a caller adds the WHERE.
const projectSelect = `SELECT p.id, p.name, p.goal, p.context, p.done_looks_like, p.done_when, p.state,
	p.target_date, p.folder_path, COALESCE(p.proposed_by_bot_id, ''), COALESCE(p.proposed_in_conversation_id, ''),
	COALESCE((SELECT c.id FROM conversations c WHERE c.project_id = p.id AND c.kind = ?
		ORDER BY c.created_at LIMIT 1), ''),
	p.created_at, p.activated_at, p.done_at, p.archived_at, p.updated_at
	FROM projects p`

// Projects is every project, oldest first; includeArchived says whether the
// archived ones are in the list.
func (db *DB) Projects(ctx context.Context, includeArchived bool) ([]project.Project, error) {
	if includeArchived {
		return db.queryProjects(ctx, ``)
	}
	return db.queryProjects(ctx, `WHERE p.state != ?`, project.Archived)
}

// Project is the project with id, or ErrNotFound.
func (db *DB) Project(ctx context.Context, id string) (project.Project, error) {
	rows, err := db.queryProjects(ctx, `WHERE p.id = ?`, id)
	for _, p := range rows {
		return p, nil
	}
	if err == nil {
		err = fmt.Errorf("project %s: %w", id, ErrNotFound)
	}
	return project.Project{}, err
}

// queryProjects reads the projects where selects, oldest first. The
// conversation kind is the first placeholder, ahead of where's.
func (db *DB) queryProjects(ctx context.Context, where string, args ...any) ([]project.Project, error) {
	args = append([]any{org.ConversationProject}, args...)
	return sqlrow.QueryAll(ctx, db.sql, "projects", scanProject,
		projectSelect+" "+where+` ORDER BY p.created_at, p.rowid`, args...)
}

func scanProject(row sqlrow.Scanner) (project.Project, error) {
	var p project.Project
	var created, activated, done, archived, updated sql.NullInt64
	err := row.Scan(&p.ID, &p.Name, &p.Brief.Goal, &p.Brief.Context, &p.Brief.DoneLooksLike, &p.Brief.DoneWhen, &p.State,
		&p.TargetDate, &p.FolderPath, &p.ProposedByBotID, &p.ProposedInConversationID, &p.ConversationID,
		&created, &activated, &done, &archived, &updated)
	p.CreatedAt, p.ActivatedAt, p.DoneAt = sqlrow.FromMillis(created), sqlrow.FromMillis(activated), sqlrow.FromMillis(done)
	p.ArchivedAt, p.UpdatedAt = sqlrow.FromMillis(archived), sqlrow.FromMillis(updated)
	return p, err
}
