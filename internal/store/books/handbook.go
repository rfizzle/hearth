package books

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/org/memory"
	"github.com/rfizzle/hearth/internal/store/sqlrow"
)

// ErrBadScope is returned by PutPage for a page whose scope is neither
// everyone nor one bot.
var ErrBadScope = errors.New("a page is for everyone or for one bot")

// Pages is every handbook page, oldest first.
func (repo *Repo) Pages(ctx context.Context) ([]memory.Page, error) {
	return sqlrow.QueryAll(ctx, repo.sql, "pages", scanPage,
		`SELECT id, title, markdown, COALESCE(scope_bot_id, ''), updated_at FROM handbook_pages ORDER BY rowid`)
}

// PutPage stores p: a page with an id replaces the page of that id, keeping
// its place in the order, and one without is added with a minted id. The
// scope must be everyone or one bot, else ErrBadScope, and the bot must
// exist, else sqlrow.ErrNotFound. It returns the page as stored.
func (repo *Repo) PutPage(ctx context.Context, p memory.Page) (memory.Page, error) {
	if p.Scope.Everyone == (p.Scope.BotID != "") {
		return memory.Page{}, fmt.Errorf("put page: %w", ErrBadScope)
	}
	p.UpdatedAt = repo.clock()
	if p.ID == "" {
		p.ID = sqlrow.NewID()
	}
	_, err := repo.sql.ExecContext(ctx, `INSERT INTO handbook_pages (id, title, markdown, scope_bot_id, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, markdown = excluded.markdown,
		scope_bot_id = excluded.scope_bot_id, updated_at = excluded.updated_at`,
		p.ID, p.Title, p.Markdown, sqlrow.NullString(p.Scope.BotID), p.UpdatedAt.UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "FOREIGN KEY") {
			return memory.Page{}, fmt.Errorf("put page: bot %s: %w", p.Scope.BotID, sqlrow.ErrNotFound)
		}
		return memory.Page{}, fmt.Errorf("put page: %w", err)
	}
	return p, nil
}

// DeletePage removes the page with id. It is sqlrow.ErrNotFound when there is none.
func (repo *Repo) DeletePage(ctx context.Context, id string) error {
	return sqlrow.DeleteByID(ctx, repo.sql, "page", "handbook_pages", id)
}

func scanPage(row sqlrow.Scanner) (memory.Page, error) {
	var p memory.Page
	var updated sql.NullInt64
	err := row.Scan(&p.ID, &p.Title, &p.Markdown, &p.Scope.BotID, &updated)
	p.Scope.Everyone = p.Scope.BotID == ""
	p.UpdatedAt = sqlrow.FromMillis(updated)
	return p, err
}
