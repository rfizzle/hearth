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
//   - A date travels as a timestamp and is kept as text in UTC. A calendar
//     day is its UTC midnight and reads back as the day, unless the control
//     asked for a time; any other instant is a date and a time. Both
//     directions agree with how the widget package parses a date.
//   - An answer with no value is the control left empty, of the control's
//     own kind, or "Other" when it carries that text, so a refusal names
//     the control rather than a kind the owner never chose.
//   - A stored spec or answer that cannot be read is treated as none, never
//     as a failure of the thread that holds it.
//
// See docs/model.md#owner-interaction.
package widgetwire
