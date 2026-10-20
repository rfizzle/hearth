// Package concern is what looks weak about a project or a hire, by rule: the
// Chief of Staff's thoughts about one field each, in fixed words.
//
// Responsibility: the six rules (Project and Bot), and Cap, the one place a
// list of concerns is bounded before it is served, whether a rule or the model
// made them. It runs no model and does no I/O; the rules are what make the
// guided sheet deterministic when no model is signed in.
//
// Invariants the types do not hold:
//   - A concern is advice, never a refusal: nothing here blocks a create, and
//     the hard refusals stay the core's.
//   - At most Max concerns come out of Cap, each about one field, each of at
//     most MaxConcern characters on one line, with a suggestion cut to the
//     field's bound. A model's concern is text from across the trust
//     boundary: it is dropped, never repaired, when its field is unknown or
//     already has a concern, or its words are empty or too long.
//   - The rules are pure: the day they compare a target date with is a
//     parameter, so a test fixes it.
//
// It imports org, project, org/memory and the standard library, so the Check
// calls can use it without a model and the drafters can return it.
// See docs/model.md#owner-interaction
package concern
