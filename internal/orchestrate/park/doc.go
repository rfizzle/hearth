// Package park is what a run that waits on the owner leaves behind, and how
// the run that continues it is handed the round it stopped in.
//
// Responsibility: turn a call a run parked on into the thread message and
// the Inbox item that ask the owner about it; word each decision for the
// thread and a denial for the bot; find in a thread the tool round a run
// parked in; and assemble the history and the prompt of the run that
// resumes it, as the round's calls and results or as one note from the app.
//
// Invariants the types do not hold:
//   - Everything here is a pure function of its arguments: no I/O, no
//     clock. The orchestrator reads and writes the store around it.
//   - The words the owner reads for a call are the tool's describer's,
//     handed in; a question's control is passed through as the JSON the
//     model sent, never parsed here.
//   - A call of the parked round with no stored result is given a fixed
//     "not run" result, so the resumed history owes the model nothing.
//
// See docs/model.md#8-approvals and docs/model.md#2-a-run.
package park
