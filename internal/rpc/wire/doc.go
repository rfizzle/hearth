// Package wire is the translation every service under rpc shares: the one
// function that picks a code for a domain error, the small helpers a handler
// answers with, the conversions more than one service needs, and the
// controls a question asks the owner to answer with.
//
// Responsibility: turn a domain error into the Connect error the app reads;
// convert the values several services carry (a bot, a tier, a provider's
// name, a credential record, a timestamp); read a stored control into the
// contract's Widget and a stored answer into its WidgetAnswer, with the
// words the bot read for it; and read an answer the owner sent into the
// widget package's value, for the orchestrator to check against the
// control. It judges nothing: the widget package refuses an answer its
// control does not allow, and every other rule stays where it is tested.
//
// Invariants the types do not hold:
//   - Fail is the one place a domain error picks its code; a handler never
//     inspects an error.
//   - A credential record crosses only in the credential calls; no event
//     body is built from one.
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
// See docs/model.md#owner-interaction and docs/architecture.md#1-layering.
package wire
