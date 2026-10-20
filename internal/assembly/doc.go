// Package assembly is the work done around a run that is not the run: tidying
// a bot's notes when they overflow.
//
// Responsibility: side runs that read the store and ask a model once, with no
// tools, and write the result back in one step.
//
// Invariants the types do not hold:
//   - A reply that is not the shape asked for changes nothing.
//
// It imports agent, framing, org/memory and store.
// See docs/model.md#7-memory-writes.
package assembly
