// Package store keeps the org in one SQLite file: the org itself, its bots
// and their grants, the conversations, their messages and the runs.
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
//
// It is handed only the path of the file.
package store
