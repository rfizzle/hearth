// Package assembly builds what a run reads, and does the work around a run
// that is not the run: the context of a bot's turn, the files the owner
// drops on a message, the conversation's running summary and the tidying of
// a bot's notes when they overflow.
//
// Responsibility: read the store and put a run's context together in a
// fixed order within byte caps (the framing and job description, the
// handbook pages the bot reads, its notes, the summary, the conversation
// window, on a resume the round it parked in, and the files attached to the
// prompt); bring dropped files where the bot may read them; and run the side
// runs that ask a model once, with no tools, and write the result back in
// one step.
//
// Invariants the types do not hold:
//   - Text the owner, a page, a file or the bot wrote enters a context only
//     inside the labelled blocks the framing names, never as framing: the
//     handbook under "## Handbook", notes one line each under
//     "## Your notes", the summary in a history entry that begins
//     "Earlier in this conversation:", attachments as "File attached:"
//     lines. A note or a file name never starts a line of its own.
//   - Each part keeps to its cap in bytes; a part over it is cut whole items
//     first and says so.
//   - A dropped path is resolved, every link followed, before it is checked
//     or copied; a folder, a missing path, or one inside a folder hearth
//     keeps for itself is refused; a copy never writes through a link.
//   - A side run's reply that is not the shape asked for changes nothing.
//
// It imports agent, framing, handbook, org, org/memory, store and
// workspace; never tools, provider or rpc.
// See docs/model.md#2-a-run and docs/model.md#7-memory-writes.
package assembly
