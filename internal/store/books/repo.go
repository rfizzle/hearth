package books

import (
	"database/sql"
	"time"
)

// Repo reads and writes notes, handbook pages, usage, budgets and tiers.
type Repo struct {
	sql   *sql.DB
	clock func() time.Time
}

// New is the repository over the open database db, stamping new rows with
// clock.
func New(db *sql.DB, clock func() time.Time) *Repo {
	return &Repo{sql: db, clock: clock}
}
