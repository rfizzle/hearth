package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rfizzle/hearth/internal/org"
)

// CreateBot stores a new bot with its grants and its DM in one transaction,
// minting the ids, and returns the bot as stored. Either all three are saved
// or none is, so no bot exists without the access it was hired with or a
// conversation to reach it in.
func (db *DB) CreateBot(ctx context.Context, bot org.Bot, grants []org.Grant, dm org.Conversation) (org.Bot, error) {
	now := db.clock()
	bot.ID = newID()
	bot.CreatedAt = now
	dm.ID = newID()
	dm.BotID = bot.ID

	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return org.Bot{}, fmt.Errorf("create bot: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO bots
		(id, name, role, job_description, tier, status, avatar, folder_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bot.ID, bot.Name, bot.Role, bot.JobDescription, bot.Tier, bot.Status, bot.Avatar, bot.FolderPath,
		now.UnixMilli()); err != nil {
		return org.Bot{}, fmt.Errorf("create bot: %w", err)
	}
	for _, g := range grants {
		if _, err := tx.ExecContext(ctx, `INSERT INTO grants (bot_id, capability, mode, folder) VALUES (?, ?, ?, ?)`,
			bot.ID, g.Capability, g.Mode, g.Folder); err != nil {
			return org.Bot{}, fmt.Errorf("create bot grant: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO conversations (id, kind, bot_id, title, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		dm.ID, dm.Kind, dm.BotID, dm.Title, now.UnixMilli(), now.UnixMilli()); err != nil {
		return org.Bot{}, fmt.Errorf("create bot dm: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return org.Bot{}, fmt.Errorf("create bot: %w", err)
	}
	bot.DMConversationID = dm.ID
	return bot, nil
}

// botColumns reads a bot with the id of its DM, which lives on the
// conversation rather than the bot.
const botColumns = `SELECT b.id, b.name, b.role, b.job_description, b.tier, b.status, b.avatar, b.folder_path,
	b.created_at, COALESCE((SELECT c.id FROM conversations c WHERE c.bot_id = b.id AND c.kind = ?
		ORDER BY c.created_at LIMIT 1), '')
	FROM bots b`

// Bots is every bot, oldest hire first.
func (db *DB) Bots(ctx context.Context) ([]org.Bot, error) {
	return queryAll(ctx, db.sql, "bots", scanBot, botColumns+` ORDER BY b.created_at, b.rowid`, org.ConversationDM)
}

// Bot is the bot with id, or ErrNotFound.
func (db *DB) Bot(ctx context.Context, id string) (org.Bot, error) {
	b, err := scanBot(db.sql.QueryRowContext(ctx, botColumns+` WHERE b.id = ?`, org.ConversationDM, id))
	if err != nil {
		return org.Bot{}, fmt.Errorf("bot %s: %w", id, notFound(err))
	}
	return b, nil
}

func scanBot(row scanner) (org.Bot, error) {
	var b org.Bot
	var created sql.NullInt64
	err := row.Scan(&b.ID, &b.Name, &b.Role, &b.JobDescription, &b.Tier, &b.Status, &b.Avatar, &b.FolderPath,
		&created, &b.DMConversationID)
	b.CreatedAt = fromMillis(created)
	return b, err
}
