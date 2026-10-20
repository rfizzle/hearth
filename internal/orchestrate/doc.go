// Package orchestrate is where work is started and followed: the runs that
// answer the owner's messages, and the event bus every change the core makes
// is published on.
//
// Responsibility: turn a message in a DM into one run of the bot's turn, and
// keep the store and the bus in step with it as it goes: the owner's
// message, the reply as it streams, each tool call and its result, the step
// count, and how the run ended. The bus gives every event the next sequence
// number of this launch; the last events are kept so a subscriber that
// reconnects is replayed what it missed, or told to resync when it missed
// more than is kept; a quiet subscription gets a heartbeat.
//
// Invariants the types do not hold:
//   - One run per conversation at a time. A message sent while one is
//     queued or working is refused, not queued.
//   - Every run ends done, cancelled or failed, and the conversation is free
//     again before the bus hears so. A failed run leaves a system message in
//     the thread saying why, written as the bot's on that run.
//   - A panic in the turn, the loop's or a tool's, ends the run failed like
//     any other failure and is logged with its stack; it never takes the core
//     down or leaves the conversation busy.
//   - A run that ends failed or cancelled with a reply that streamed nothing
//     deletes that empty message rather than leaving a bubble. Launch also
//     closes any message an earlier launch left streaming.
//   - A message is never left in the thread without a run to answer it: if the
//     run cannot be stored, the message is removed again.
//   - A cancelled run keeps what streamed: its writes do not use the context
//     that stopped it.
//   - Deltas go to the bus only. A streaming message is stored empty when it
//     opens and whole when it closes.
//   - Every tool call goes through the dispatcher with the bot's grants. The
//     tools are built for each run, so a search key set after launch counts
//     from the next message.
//   - Publish never waits on a subscriber. One that falls a full buffer
//     behind is dropped, its channel closed, and it reconnects from the last
//     sequence it read.
//
// Seams it is handed: the store, a function from a tier to a stream, the
// scrub, the page fetcher, a function that says whether a search service is
// configured, the bus, the caps and a clock. It imports no provider and no
// integration, so a test hands it a script.
package orchestrate
