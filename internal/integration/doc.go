// Package integration keeps the optional services that are not model
// providers, today one web-search key, and decides which backend a run
// searches with.
//
// Responsibility: list the known integrations with whether each holds a key,
// store and clear a key through the shared KeyStore, tell the handlers about
// each change, and build the searcher a run is handed. The table of services
// is the one place a new service is named: one constant and one backend
// constructor.
//
// Invariants the types do not hold:
//   - A key is never returned, logged or put in an error. An account reports
//     the last four characters of a key, never the key.
//   - No key, no tool: Searcher is nil unless a key is stored. It is built
//     fresh on each call, so a key set or cleared after wiring takes effect
//     on the next run.
//   - The key lives in the provider's KeyStore under the integration's name,
//     so the provider's Scrub already redacts it from tool results.
//   - A name outside the table is refused before the store is touched.
//
// Seams it is handed: the KeyStore it shares with provider, an HTTP client
// for the backends, and OnChange, which hears every account change. It
// returns its own types; the contract handlers convert them.
// See docs/decisions.md#hearth--decisions
package integration
