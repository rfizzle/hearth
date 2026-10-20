// Package inbox holds what waits for the owner: the items in the Inbox, the
// four decisions on an approval, the allowances a decision can leave behind,
// and the rule that turns a decision into grants.
//
// Responsibility: plain types and one pure rule. The orchestrator decides
// when to open and close an item; the store keeps the rows.
//
// Invariants the types do not hold:
//   - The package imports org and the standard library only, and does no I/O.
//   - The numeric value of each enum is stored on disk, so a value is never
//     renumbered; a new one is appended.
//   - Apply never changes the grants it is given, and never widens access
//     past what the decision names.
//
// See docs/model.md#8-approvals.
package inbox
