// Package provider is hearth's one door to a model: it names the app to llm,
// keeps the person's accounts, signs them in, and says which model answers a
// quality tier.
//
// Responsibility: own every import of github.com/rfizzle/llm outside the agent
// loop. The helper's commands and the orchestrator call this package and never
// llm directly. It knows which providers are signed in, builds the first one
// for a tier, opens a completion stream in llm's terms, and scrubs every
// stored secret out of a message.
//
// Invariants the types do not hold:
//   - Credentials live in owner-only files under the person's application
//     support folder, a 0700 directory of 0600 files, and nowhere else. The
//     core never prints a key or token, never returns one over an interface
//     and never writes one into the database. An account reports the last
//     four characters of a key, never the key.
//   - The key store is shared by name with whoever else keeps a secret (a
//     search service, say), so Scrub reads every name in it, not only the
//     provider keys.
//   - A returning launch reads the stored sign-in; it opens no browser and
//     asks for nothing. A browser sign-in started for the app returns the
//     page at once and finishes in the background; at most one is in flight.
//   - A signed-out account is reported in words that name what to do, not the
//     error chain.
//
// Seams it is handed: the http client that reaches the authorization server,
// the function that opens the browser, the directory the credentials live in,
// the KeyStore, and OnChange, which hears every account change.
//
// The file store is a stopgap. The planned follow-on is a Keychain-backed
// store behind the same KeyStore interface and the same token store seam in
// llm, so the credentials leave the disk and the core holds them in memory.
// See docs/architecture.md#2-responsibilities
package provider
