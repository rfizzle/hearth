// Package sqlrow is what the store's repositories share for reading and
// writing rows: the minted id, the time columns, a query read into values,
// a one-row update or delete, and a transaction.
//
// Responsibility: the row-level mechanics, so each repository holds only
// its own queries and scans.
//
// Invariants the types do not hold:
//   - ErrNotFound is the one not-found error every repository returns, so a
//     caller tests for it once whichever package answered.
//   - Times are Unix milliseconds in the file, and the zero time is NULL.
//
// It is handed an open database and imports no driver.
package sqlrow
