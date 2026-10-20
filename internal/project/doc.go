// Package project holds a project's types and the one rule over them: the
// status derived from its tasks.
//
// Responsibility: say what a project is, validate and build one, and turn a
// count of its tasks into the light and the label the owner sees.
//
// Invariants the types do not hold:
//   - StatusOf is the one rule for a project's light; the sidebar and any
//     later measurement read it and never derive their own.
//   - A proposed project has no folder and no conversation; making it active
//     gives it both and keeps its id and its proposer.
//   - A project's folder is fixed when it is made: a rename never moves it.
//   - The stored values of State and Light are written to disk and to the
//     wire; they are never renumbered.
//
// It does no I/O and mints no ids; the store mints them and the caller makes
// the folder. It imports org, org/memory and the standard library only.
// See docs/model.md#work.
package project
