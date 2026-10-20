// Package skill is the role templates hearth ships: a starting point for
// hiring, written as SKILL.md bundles so they load the way agentcore loads
// any skill and can be shared the same way later.
//
// Responsibility: embed the shipped templates and hand them out as org
// values. The skill name is the template id, its description the summary,
// its role, suggested tier and scope come from its metadata, and its body is
// the job description a new bot gets. The team templates are the ones the
// owner hires from; the org-level Chief of Staff is hired for the owner at
// onboarding. Every template's frontmatter carries the introduction a bot
// hired from it opens with, under an introduction key agentcore's parser
// passes over; this package reads it itself.
//
// Invariants the types do not hold:
//   - A template that does not parse, or names a tier that is not one of
//     fast, balanced or best, or a scope that is not team or org, is a fault
//     in the binary, so it panics; a test loads every one.
//   - A template without a valid introduction (a purpose, three how lines
//     with a symbol each, three short prompts) is the same fault.
//   - Templates come in name order, so the list the app shows is stable.
package skill
