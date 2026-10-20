// Package bus is the event bus every change the core makes is published on,
// and the bodies of the events the orchestrator publishes.
//
// Responsibility: give every event the next sequence number of this launch,
// keep the latest so a subscriber that reconnects is replayed what it
// missed, or told to resync when it missed more than is kept, and send a
// quiet subscription a heartbeat.
//
// Invariants the types do not hold:
//   - Publish never waits on a subscriber. One that falls a full buffer
//     behind is dropped, its channel closed, and it reconnects from the last
//     sequence it read.
//   - Deltas are never kept as messages; a streaming message's final upsert
//     carries its whole text.
//
// It imports org and org/inbox for the event bodies, and nothing else of
// hearth's.
package bus
