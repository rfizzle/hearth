// Package after is the work a run that ended done leaves behind.
//
// Responsibility: tidy a bot's notes when they have reached the cap and fold
// the text a run's window left out into the conversation's summary, both
// through the Fast tier's model, with the usage counted against the run
// that caused them, and publish what changed.
//
// Invariants the types do not hold:
//   - A tidy holds the bot's conversation while it works, so no run writes a
//     note between the read and the replace; if a run holds it already, the
//     tidy waits for the next one.
//   - One summary is written at a time per conversation; a second is left to
//     the run after.
//   - A failure is logged and never reaches the run, which has ended.
//
// It is handed the claim on a conversation as functions, so it imports no
// part of the service.
// See docs/model.md#7-memory-writes.
package after
