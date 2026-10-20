// Package memory is the tool a bot writes its notes with: remember, one
// plain sentence at a time, linked to the message it was learned in.
//
// Responsibility: the definition the model reads, the check that refuses an
// empty or oversized note, the words the owner sees for it, and the reply
// that says when the notes are full.
//
// Invariants the types do not hold:
//   - The notes are reached only through the functions in Store, which the
//     orchestrator builds for one bot, conversation and message; nothing here
//     reads the store, and a note carries no bot but the one in Store.
//   - A full set of notes still takes the note: the cap is met by tidying
//     after the reply, never by refusing the bot.
//
// It imports tools, org and org/memory.
// See docs/model.md#7-memory-writes.
package memory
