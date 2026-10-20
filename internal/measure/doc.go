// Package measure is the office's own figures, read by id: a project's tasks
// and light, what a bot has cost in a month, the Inbox count, a bot's notes,
// the handbook's meter and the size of the team.
//
// Responsibility: a closed registry of ids, "family/subject[/period]", each
// with one of three value kinds (a number, a progress or a light). ParseID
// reads an id in words, Resolve turns the aliases this, me, org and current
// into the ids they mean for one caller, Read returns one value per resolved
// id and List the ids that resolve now. The tool that writes a dashboard and
// the app's live bindings are its two callers; neither types a number the
// office keeps.
//
// Invariants the types do not hold:
//   - The registry is append-only (families.go): an id a saved dashboard
//     holds is never renamed or given another meaning.
//   - A value is computed by a rule, never by a model, and carries the time
//     it was read.
//   - A subject that is gone, or a figure that cannot be known (a cost with
//     an unpriced request, a budget that is not set), is a Value with Missing
//     set in words, never an error; an error is a failed read.
//   - Aliases are resolved once, at write time. Read refuses an id that still
//     holds one, so a value never depends on who asks.
//   - The package imports org, project and the standard library only. The
//     store, the meter and the handbook reach it through the small
//     interfaces of Deps, which hearthd satisfies.
//
// Seams it is handed (Deps): the bots, the projects and a project's status,
// a month of usage, the Inbox's open count, a bot's note count, the
// handbook's meter, and the clock, so a test pins the month and the read
// time. See docs/model.md#work.
package measure
