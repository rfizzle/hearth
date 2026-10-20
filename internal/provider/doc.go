// Package provider is hearth's one door to a model: it names the app to llm,
// keeps the person's accounts, signs them in, and says which model answers a
// quality tier.
//
// Responsibility: own every import of github.com/rfizzle/llm outside the agent
// loop. The helper's commands and the orchestrator call this package and never
// llm directly. It knows which providers are signed in, keeps the
// OpenAI-compatible services the person added by address as named profiles,
// builds the one a tier resolves to, opens a completion stream in llm's
// terms, and scrubs every stored secret out of a message. The network side
// of a service by address (the table of known services, the address check,
// the model list) is the sub-package gateway.
//
// Invariants the types do not hold:
//   - Once the app has synced its Keychain (Sync), credentials live in
//     keystore.MemoryKeys and nowhere else: no path writes one to disk. Only a core
//     run by hand with no app keeps them in owner-only files under the
//     person's application support folder, a 0700 directory of 0600 files,
//     and those files are deleted by the second Sync that holds every name
//     they held (D29). The core never prints a key or token, never writes
//     one into the database or a log, and returns one over an interface in
//     two places only: Sync's records to store and Export. An account
//     reports the last four characters of a key, never the key.
//   - The key store is shared by name with whoever else keeps a secret (a
//     search service, say), so Scrub reads every name in it, not only the
//     provider keys.
//   - A returning launch reads the stored sign-in; it opens no browser and
//     asks for nothing. A browser sign-in started for the app returns the
//     page at once and finishes in the background; at most one is in flight.
//   - A signed-out account is reported in words that name what to do, not the
//     error chain.
//   - An added service's key is stored under its own name and its settings
//     (name, address, model) beside it under another, so the key is never in
//     the settings; the settings are not a secret and the scrub skips them.
//     A key is never sent to another machine over plain http.
//   - An added service that ends a completion with nothing said, from a body
//     with no choices in it, is an error naming the service and quoting the
//     first of the body with secrets removed, never a silent empty reply.
//   - Saving a service first asks its address for models with its key and
//     refuses, in plain words, anything that is not a model list; nothing
//     is stored then. The words never hold the key or what the service sent.
//
// Seams it is handed: the http client that reaches the authorization server,
// the function that opens the browser, the directory the credentials live in,
// the keystore.Store, and OnChange, which hears every account change.
//
// The file stores are for a hand-run core. The app pushes its Keychain
// records, and keystore.MemoryKeys serves as both the keystore.Store and llm's
// credstore.Store behind routers that Sync switches, so holders of
// Accounts.Keys need not be told.
// See docs/architecture.md#2-responsibilities
package provider
