// Package skill is the role templates hearth ships: a starting point for
// hiring, written as SKILL.md bundles so they load the way agentcore loads
// any skill and can be shared the same way later.
//
// Responsibility: embed the shipped templates and hand them out as org
// values. The skill name is the template id, its description the summary,
// its role and suggested tier come from its metadata, and its body is the
// job description a new bot gets.
//
// Invariants the types do not hold:
//   - A template that does not parse, or names a tier that is not one of
//     fast, balanced or best, is a fault in the binary, so it panics; a test
//     loads every one.
//   - Templates come in name order, so the list the app shows is stable.
package skill
