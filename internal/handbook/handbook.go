package handbook

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rfizzle/hearth/internal/org/memory"
)

// ErrFull is returned by Put for a page that would take a reader's handbook
// past the limit.
var ErrFull = errors.New("the handbook is full: shorten a page or remove one to make room")

// ErrNoTitle is returned by Put for a page with no title.
var ErrNoTitle = errors.New("a page needs a title")

// Store is the page storage the service needs; the database satisfies it.
type Store interface {
	Pages(ctx context.Context) ([]memory.Page, error)
	PutPage(ctx context.Context, p memory.Page) (memory.Page, error)
	DeletePage(ctx context.Context, id string) error
}

// Meter is how full a reader's handbook is, in bytes of Markdown.
type Meter struct {
	Used int64
	Cap  int64
}

// Service keeps the handbook pages. OnChange, when set, is called after a
// page is stored or removed with the page and whether it was deleted.
type Service struct {
	Store    Store
	OnChange func(page memory.Page, deleted bool)
}

// List is the pages the bot with botID reads, everyone's first, and how full
// they make its handbook. With an empty botID it is the pages for everyone.
func (s *Service) List(ctx context.Context, botID string) ([]memory.Page, Meter, error) {
	all, err := s.Store.Pages(ctx)
	if err != nil {
		return nil, Meter{}, err
	}
	seen := memory.Select(all, botID)
	used, limit := memory.Meter(seen)
	return seen, Meter{Used: used, Cap: limit}, nil
}

// Put stores p and returns it as stored. A page with an id replaces the page
// of that id. It refuses an empty title and a page that takes the view of any
// bot it reaches over the cap.
func (s *Service) Put(ctx context.Context, p memory.Page) (memory.Page, error) {
	if strings.TrimSpace(p.Title) == "" {
		return memory.Page{}, ErrNoTitle
	}
	all, err := s.Store.Pages(ctx)
	if err != nil {
		return memory.Page{}, err
	}
	after := make([]memory.Page, 0, len(all)+1)
	replaced := false
	for _, q := range all {
		if p.ID != "" && q.ID == p.ID {
			after = append(after, p)
			replaced = true
			continue
		}
		after = append(after, q)
	}
	if !replaced {
		after = append(after, p)
	}
	for _, id := range readers(after, p) {
		if used, limit := memory.Meter(memory.Select(after, id)); used > limit {
			return memory.Page{}, fmt.Errorf("put page: %w", ErrFull)
		}
	}
	stored, err := s.Store.PutPage(ctx, p)
	if err != nil {
		return memory.Page{}, err
	}
	if s.OnChange != nil {
		s.OnChange(stored, false)
	}
	return stored, nil
}

// readers is the bots whose view p is in: its own bot, or, for a page for
// everyone, each bot any page is for, and the reader with no pages of its own.
func readers(pages []memory.Page, p memory.Page) []string {
	if !p.Scope.Everyone {
		return []string{p.Scope.BotID}
	}
	ids := []string{""}
	seen := map[string]bool{"": true}
	for _, q := range pages {
		if id := q.Scope.BotID; !q.Scope.Everyone && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// Delete removes the page with id.
func (s *Service) Delete(ctx context.Context, id string) error {
	all, err := s.Store.Pages(ctx)
	if err != nil {
		return err
	}
	if err := s.Store.DeletePage(ctx, id); err != nil {
		return err
	}
	if s.OnChange != nil {
		page := memory.Page{ID: id}
		for _, q := range all {
			if q.ID == id {
				page = q
			}
		}
		s.OnChange(page, true)
	}
	return nil
}

// Render is the "## Handbook" block for pages in the order given: each as a
// "### <title>" heading and its Markdown, as many whole pages as fit in the
// cap, then a notice naming each page left out. It is empty without pages.
func Render(pages []memory.Page) string {
	if len(pages) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Handbook\n")
	var used int64
	var left []string
	for _, p := range pages {
		n := int64(len(p.Markdown))
		if used+n > memory.HandbookCap {
			left = append(left, p.Title)
			continue
		}
		used += n
		fmt.Fprintf(&b, "\n### %s\n\n%s\n", p.Title, strings.TrimRight(p.Markdown, "\n"))
	}
	if len(left) > 0 {
		fmt.Fprintf(&b, "\nLeft out because the handbook is full: %s.\n", strings.Join(left, ", "))
	}
	return b.String()
}
