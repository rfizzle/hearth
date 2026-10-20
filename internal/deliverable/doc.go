// Package deliverable holds the spec of a shaped deliverable, a dashboard or a
// report a bot fills and Hearth renders, and the rules that turn a malformed
// one back in words.
//
// Responsibility: plain types and pure rules. A Spec is version 1 JSON of
// sections that hold widgets from a closed catalogue of ten (metric, line, bar,
// pie, table, list, progress, light, callout, text). Parse reads one strictly,
// Validate refuses what the catalogue does not allow, Canonical writes the one
// form the renderers and the goldens rely on, and Format is the one number
// formatter.
//
// Invariants the types do not hold:
//   - The package imports the standard library and goldmark only, does no I/O
//     and keeps no clock.
//   - The catalogue is closed. A kind is added here, in the renderers and in the
//     app, and nowhere else.
//   - Every refusal wraps ErrInvalid and says what to change and where, because
//     the model reads it and tries again.
//   - Parse and Validate run to the end before any renderer is called; a
//     renderer takes a Spec it treats as valid.
//   - Markdown in a text or callout widget has no HTML, no image and no link
//     but http, https and mailto; ParseMarkdown is the one strict parser every
//     renderer shares.
//   - A measure is only an id here. Whether it names a real family and subject,
//     and what value it reads, is the measure layer's to say.
//
// See docs/model.md#work.
package deliverable
