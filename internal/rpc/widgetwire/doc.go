// Package widgetwire turns the control a question asks the owner to answer
// with, and the owner's answer to it, between the form the core stores and
// the form the contract carries.
//
// Responsibility: read a stored control into the contract's Widget; read a
// stored answer into its WidgetAnswer, with the words the bot read for it;
// and read an answer the owner sent into the widget package's value, for the
// orchestrator to check against the control. It judges nothing: the widget
// package refuses an answer its control does not allow.
//
// Invariants the types do not hold:
//   - A date travels as a timestamp and is kept as text in the Mac's own
//     zone, the zone of the owner's date picker and of the bounds a bot
//     writes. A calendar day is its local midnight and reads back as the
//     day, unless the control asked for a time; any other instant is a date
//     and a time on the local clock. The core and the app run on one Mac, so
//     both read the same day and time.
//   - Words the owner types beside a control, and a reason for a refusal,
//     are one line of at most 4,096 characters.
//   - An answer with no value is the control left empty, of the control's
//     own kind, or "Other" when it carries that text, so a refusal names
//     the control rather than a kind the owner never chose.
//   - A stored spec or answer that cannot be read is treated as none, never
//     as a failure of the thread that holds it.
//
// See docs/model.md#owner-interaction.
package widgetwire
