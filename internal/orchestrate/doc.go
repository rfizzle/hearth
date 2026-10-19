// Package orchestrate is where work is started and followed: the runs that
// answer the owner's messages, and the event bus every change the core makes
// is published on.
//
// Responsibility: own the bus the app follows. Every event gets the next
// sequence number of this launch; the last events are kept so a subscriber
// that reconnects is replayed what it missed, or told to resync when it
// missed more than is kept; a quiet subscription gets a heartbeat.
//
// Invariants the types do not hold:
//   - Publish never waits on a subscriber. One that falls a full buffer
//     behind is dropped, its channel closed, and it reconnects from the last
//     sequence it read.
//   - Sequence numbers restart at 1 with every launch.
//
// It is handed nothing; its callers publish to it and subscribe to it.
package orchestrate
