package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/org/memory"
)

// ErrBadScope is returned by PutPage for a page whose scope is neither
// everyone nor one bot.
var ErrBadScope = errors.New("a page is for everyone or for one bot")

// Pages is every handbook page, oldest first.
func (db *DB) Pages(ctx context.Context) ([]memory.Page, error) {
	return queryAll(ctx, db.sql, "pages", scanPage,
		`SELECT id, title, markdown, COALESCE(scope_bot_id, ''), updated_at FROM handbook_pages ORDER BY rowid`)
}

// PutPage stores p: a page with an id replaces the page of that id, keeping
// its place in the order, and one without is added with a minted id. The
// scope must be everyone or one bot, else ErrBadScope, and the bot must
// exist, else ErrNotFound. It returns the page as stored.
func (db *DB) PutPage(ctx context.Context, p memory.Page) (memory.Page, error) {
	if p.Scope.Everyone == (p.Scope.BotID != "") {
		return memory.Page{}, fmt.Errorf("put page: %w", ErrBadScope)
	}
	p.UpdatedAt = db.clock()
	if p.ID == "" {
		p.ID = newID()
	}
	_, err := db.sql.ExecContext(ctx, `INSERT INTO handbook_pages (id, title, markdown, scope_bot_id, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, markdown = excluded.markdown,
		scope_bot_id = excluded.scope_bot_id, updated_at = excluded.updated_at`,
		p.ID, p.Title, p.Markdown, nullString(p.Scope.BotID), p.UpdatedAt.UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return memory.Page{}, fmt.Errorf("put page: bot %s: %w", p.Scope.BotID, ErrNotFound)
		}
		return memory.Page{}, fmt.Errorf("put page: %w", err)
	}
	return p, nil
}

// DeletePage removes the page with id. It is ErrNotFound when there is none.
func (db *DB) DeletePage(ctx context.Context, id string) error {
	return db.deleteByID(ctx, "page", "handbook_pages", id)
}

func scanPage(row scanner) (memory.Page, error) {
	var p memory.Page
	var updated sql.NullInt64
	err := row.Scan(&p.ID, &p.Title, &p.Markdown, &p.Scope.BotID, &updated)
	p.Scope.Everyone = p.Scope.BotID == ""
	p.UpdatedAt = fromMillis(updated)
	return p, err
}
