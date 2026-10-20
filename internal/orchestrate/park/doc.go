// Package park is what a run that waits on the owner leaves behind.
//
// Responsibility: turn a call a run parked on into the thread message and
// the Inbox item that ask the owner about it; word each decision for the
// thread and a denial for the bot, and raise a grant for an always-allow;
// turn the owner's answer to a question into their words for the thread and
// the fixed words the bot reads; turn a run a provider failed into the
// Inbox item the owner retries or acknowledges. Finding the round a run
// parked in, and handing it to the run that resumes, is assembly's.
//
// Invariants the types do not hold:
//   - Everything here is a pure function of its arguments: no I/O, no
//     clock. The orchestrator reads and writes the store around it.
//   - The words the owner reads for a call are the tool's describer's,
//     handed in; a question's control is passed through as the JSON the
//     model sent, and read here only to check the owner's answer against
//     it and to word that answer.
//
// See docs/model.md#8-approvals.
package park
