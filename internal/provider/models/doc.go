// Package models says which model a bot runs on and which models there are
// to choose: the owner's tier mapping, a model one bot pins, the list of
// models the accounts offer, and the probe for a runtime on this Mac.
//
// Responsibility: choose among models and list them. It builds nothing and
// stores nothing; provider builds the model and the caller reads the mapping
// from wherever it is kept and hands it in.
//
// Invariants the types do not hold:
//   - An override wins over the mapping, the mapping over the defaults. A
//     mapping row whose provider is gone falls back to the defaults; an
//     override whose provider is gone is an error, since a model the owner
//     pinned is never swapped for another without telling them.
//   - Detect reaches only the loopback addresses in the known-services table,
//     and waits at most half a second for each.
//
// See docs/model.md#providers-and-models
package models
