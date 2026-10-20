// Package store keeps the org in one SQLite file: the org itself, its bots
// and their grants, the conversations, their messages, the runs, the Inbox
// items waiting for the owner and the allowances a decision leaves. What the
// bots remember, read and cost (notes, handbook pages, usage, budgets and
// tiers) is the books sub-package's repository, which the store embeds, and
// the row mechanics every repository shares are in sqlrow.
//
// Responsibility: open the file, bring its schema up to date, and read and
// write org values. Every other package reads and writes the org through
// here, and gets org types back.
//
// Invariants the types do not hold:
//   - This is the only package that imports the SQLite driver.
//   - One open connection, WAL, foreign keys on, and every transaction takes
//     the write lock when it begins, so two writers wait in turn rather than
//     fail half-way.
//   - The migration list is append-only. A released step is never edited,
//     removed or moved; a change is a new step at the end.
//   - Ids are minted here, when a value is first stored. Values built in org
//     carry none until then.
//   - No credential is stored in any table.
//   - Two bots, or a bot and a project, never share a folder, compared
//     without regard to case; the second is refused inside the transaction
//     that would save it.
//   - The org's first save hires its Chief of Staff, an org-level bot, in
//     the same transaction, and a later save hires nobody.
//
// Coordinator is the seam the projects work resolves its coordinator
// through: the org-level bot a project conversation is addressed to, or
// ErrNotFound while there is none.
//
// It is handed only the path of the file.
package store
