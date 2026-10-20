// Package meter is hearth's usage ledger: it prices each model request from
// llm's price table, keeps the month's totals, and enforces the owner's
// budgets, telling the owner at 80% and pausing the bot (or the org) at 100%.
//
// Responsibility: pricing, recording, the month summary and the budget
// rails. The rows live in the store and the pure rules in org/usage; the
// meter acts on them.
//
// Invariants:
//   - The package imports the store, org/usage, org/inbox and llm's pricing
//     only. It never imports orchestrate: what it must do to a run, it asks
//     of the OnPause function it is given.
//   - Money is whole micro-dollars. A request on a plan, or one whose model
//     the table lacks, costs 0 and is never guessed at; its tokens count.
//   - A subject paused in a month stays paused until Resume, or a budget
//     above the spend, or the month changes. A subject resumed in a month is
//     not paused again that month until its budget is raised.
//   - The soft item is created once per subject per month and budget.
//
// See docs/model.md#usage.
package meter
