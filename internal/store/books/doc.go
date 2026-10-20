// Package books keeps what the bots remember, read and cost in the store's
// SQLite file: each bot's notes, the handbook's pages, the usage of every
// model request, the budgets and the tier mapping.
//
// Responsibility: read and write those rows and hand back org values. The
// store opens the file and embeds a Repo, so a caller reaches these through
// the store like any other repository.
//
// Invariants the types do not hold:
//   - A pinned note survives a replace; the replace is one transaction.
//   - A handbook page is for everyone or for one bot, never both or neither.
//   - A missing row is sqlrow.ErrNotFound, the store's own not-found error.
//
// It is handed the open database and the store's clock, and imports no
// driver.
package books
