// Package widget holds the closed catalogue of questions a bot may ask the
// owner with a control instead of prose: a single or multiple choice, free
// text, yes or no, a date, a file or folder, and a scale.
//
// Responsibility: plain types and pure rules. A Spec says what to ask, an
// Answer says what the owner gave, Validate refuses a malformed Spec, Check
// refuses an Answer the Spec does not allow, and Render turns an Answer into
// the fixed words the bot reads.
//
// Invariants the types do not hold:
//   - The package imports the standard library only and does no I/O.
//   - The catalogue is closed. A kind is added here, as a proto case and as a
//     control in the app, and nowhere else.
//   - Every refusal wraps ErrInvalid and says what to change, because the
//     model reads it and tries again.
//   - Render is the only way an answer reaches the bot; it never passes data.
//
// A question that carries a control pauses the run like any other, and the
// answer resumes it. See docs/model.md#owner-interaction.
package widget
