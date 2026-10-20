// Package park is what a run that waits on the owner leaves behind, and how
// a run is handed the conversation, the run that continues one with the
// round it stopped in.
//
// Responsibility: turn a call a run parked on into the thread message and
// the Inbox item that ask the owner about it; word each decision for the
// thread and a denial for the bot, and raise a grant for an always-allow;
// turn the owner's answer to a question into their words for the thread and
// the fixed words the bot reads; turn a run a provider failed into the
// Inbox item the owner retries or acknowledges; find in a thread the tool
// round a run
// parked in; and assemble the history and the prompt a run is handed, the
// text that fits its budget and, for a run that resumes, the round's calls
// and results or one note from the app.
//
// Invariants the types do not hold:
//   - Everything here is a pure function of its arguments: no I/O, no
//     clock. The orchestrator reads and writes the store around it.
//   - The words the owner reads for a call are the tool's describer's,
//     handed in; a question's control is passed through as the JSON the
//     model sent, and read here only to check the owner's answer against
//     it and to word that answer.
//   - A call of the parked round with no stored result is given a fixed
//     "not run" result, so the resumed history owes the model nothing.
//
// See docs/model.md#8-approvals and docs/model.md#2-a-run.
package park
