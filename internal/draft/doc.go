// Package draft turns a few words from the owner into a proposed bot: a name,
// a role, a job description in plain sentences, a one-line working style and a
// tier; or a paragraph into a proposed project: a name, a four-part brief and
// a target date. It makes one model call outside any conversation and returns
// what came back; the owner reads it, changes it and decides.
//
// Responsibility: build the one request, run it with no tools and parse the
// reply. It is a package of its own rather than part of orchestrate because
// a run there belongs to a conversation, a bot and the store, and a draft has
// none of the three.
//
// Invariants the types do not hold:
//   - The reply is a single JSON object with the fields name, role,
//     job_description, working_style and tier. The parser takes the text from
//     the first opening brace to the last closing one, so a code fence or a
//     sentence around the object does not matter; anything that is not
//     exactly one object with a job description is ErrMalformed, and so is a
//     field it does not know, a value of the wrong type or a second value
//     after it. A missing name, role, style or tier is left empty (the tier
//     then falls back to the one asked for). A project's reply is the same
//     with name, goal, context, done_looks_like, done_when and target_date;
//     the goal and done_when are required and a target_date that is not a
//     day is emptied.
//   - Every string is bounded before it leaves (bound.go): trimmed, a
//     one-line field made one line, and cut at its bound in characters. A
//     cut, not a refusal.
//   - No tool is offered and no executor is installed, so nothing the model
//     says is ever run.
//   - The owner's description is sent as it was typed (after the scrub the
//     caller supplies) and the result is only a proposal. This package never
//     stores or changes anything.
//
// Both errors read as a sentence to a person, for the app to show as it is.
// See docs/model.md#people.
//
// It imports agent, framing, org and project, and concern below it, never
// store, rpc or tools.
package draft
