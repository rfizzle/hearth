package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/org"
	"github.com/rfizzle/hearth/internal/org/intro"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// CreateBot stores a new bot with its grants and its DM in one transaction,
// minting the ids, and returns the bot as stored. Either all three are saved
// or none is, so no bot exists without the access it was hired with or a
// conversation to reach it in.
func (db *DB) CreateBot(ctx context.Context, bot org.Bot, grants []org.Grant, dm org.Conversation) (org.Bot, error) {
	err := sqlrow.InTx(ctx, db.sql, "create bot", func(tx *sql.Tx) (err error) {
		bot, err = db.insertBot(ctx, tx, bot, grants, dm)
		return err
	})
	if err != nil {
		return org.Bot{}, err
	}
	return bot, nil
}

// insertBot stores bot, its grants and its DM inside tx, refusing a folder
// another bot has, and returns the bot with its ids. A bot with no scope is
// a team's.
func (db *DB) insertBot(ctx context.Context, tx *sql.Tx, bot org.Bot, grants []org.Grant, dm org.Conversation) (org.Bot, error) {
	now := db.clock()
	bot.ID = sqlrow.NewID()
	bot.CreatedAt = now
	if bot.Scope == 0 {
		bot.Scope = org.ScopeTeam
	}
	dm.ID = sqlrow.NewID()
	dm.BotID = bot.ID
	if err := checkFolderFree(ctx, tx, bot.FolderPath); err != nil {
		return org.Bot{}, err
	}
	if bot.Introduction.Purpose == "" {
		bot.Introduction = intro.Assemble(bot.Role, bot.JobDescription)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO bots
		(id, name, role, job_description, tier, status, avatar, folder_path, created_at, scope, introduction)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bot.ID, bot.Name, bot.Role, bot.JobDescription, bot.Tier, bot.Status, bot.Avatar, bot.FolderPath,
		now.UnixMilli(), bot.Scope, introductionJSON(bot.Introduction)); err != nil {
		return org.Bot{}, err
	}
	for _, g := range grants {
		if _, err := tx.ExecContext(ctx, `INSERT INTO grants (bot_id, capability, mode, folder) VALUES (?, ?, ?, ?)`,
			bot.ID, g.Capability, g.Mode, g.Folder); err != nil {
			return org.Bot{}, fmt.Errorf("grant: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversations (id, kind, bot_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		dm.ID, dm.Kind, dm.BotID, dm.Title, now.UnixMilli(), now.UnixMilli()); err != nil {
		return org.Bot{}, fmt.Errorf("dm: %w", err)
	}
	bot.DMConversationID = dm.ID
	return bot, nil
}

// checkFolderFree is ErrFolderTaken when another bot's or a project's folder
// is folder, compared without regard to case: the default Mac volume is
// case-insensitive, so "Dana" and "dana" would share one folder on disk.
// It reads inside the creating transaction, so two hires racing for one
// name cannot both pass.
// See docs/model.md#access
func checkFolderFree(ctx context.Context, tx *sql.Tx, folder string) error {
	rows, err := tx.QueryContext(ctx, `SELECT folder_path FROM bots UNION ALL SELECT folder_path FROM projects WHERE folder_path != ''`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var other string
		if err := rows.Scan(&other); err != nil {
			return err
		}
		if strings.EqualFold(other, folder) {
			return ErrFolderTaken
		}
	}
	return rows.Err()
}

// botColumns reads a bot with the id of its DM, which lives on the
// conversation rather than the bot.
const botColumns = `SELECT b.id, b.name, b.role, b.job_description, b.tier, b.status, b.avatar, b.folder_path,
	b.created_at, b.model_override, b.scope, b.introduction, COALESCE((SELECT c.id FROM conversations c WHERE c.bot_id = b.id AND c.kind = ?
		ORDER BY c.created_at LIMIT 1), '')
	FROM bots b`

// Bots is every bot, oldest hire first.
func (db *DB) Bots(ctx context.Context) ([]org.Bot, error) {
	return sqlrow.QueryAll(ctx, db.sql, "bots", scanBot, botColumns+` ORDER BY b.created_at, b.rowid`, org.ConversationDM)
}

// Bot is the bot with id, or ErrNotFound.
func (db *DB) Bot(ctx context.Context, id string) (org.Bot, error) {
	b, err := scanBot(db.sql.QueryRowContext(ctx, botColumns+` WHERE b.id = ?`, org.ConversationDM, id))
	if err != nil {
		return org.Bot{}, fmt.Errorf("bot %s: %w", id, sqlrow.NotFound(err))
	}
	return b, nil
}

// UpdateBot sets what the owner may edit on the bot with bot.ID: its name,
// role, job description, tier, avatar and model override. Its folder,
// status and grants do not change here. It returns the bot as stored, or
// ErrNotFound.
func (db *DB) UpdateBot(ctx context.Context, bot org.Bot) (org.Bot, error) {
	if err := sqlrow.Update(ctx, db.sql, "bot "+bot.ID, `UPDATE bots
		SET name = ?, role = ?, job_description = ?, tier = ?, avatar = ?, model_override = ? WHERE id = ?`,
		bot.Name, bot.Role, bot.JobDescription, bot.Tier, bot.Avatar, bot.ModelOverride, bot.ID); err != nil {
		return org.Bot{}, err
	}
	return db.Bot(ctx, bot.ID)
}

// introductionJSON is i as stored. Strings and slices of them always encode,
// so there is no error to return.
func introductionJSON(i intro.Introduction) []byte {
	b, _ := json.Marshal(i)
	return b
}

func scanBot(row sqlrow.Scanner) (org.Bot, error) {
	var b org.Bot
	var created sql.NullInt64
	var introduction string
	err := row.Scan(&b.ID, &b.Name, &b.Role, &b.JobDescription, &b.Tier, &b.Status, &b.Avatar, &b.FolderPath,
		&created, &b.ModelOverride, &b.Scope, &introduction, &b.DMConversationID)
	b.CreatedAt = sqlrow.FromMillis(created)
	// A bot hired before introductions existed, or one whose stored text
	// cannot be read, opens with the one its role and job description give.
	if json.Unmarshal([]byte(introduction), &b.Introduction) != nil || b.Introduction.Purpose == "" {
		b.Introduction = intro.Assemble(b.Role, b.JobDescription)
	}
	return b, err
}
