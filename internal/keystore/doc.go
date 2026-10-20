// Package keystore holds the core's secrets by name: the Store interface, the
// owner-only file store a hand-run core uses, and the memory store the core
// uses once the app has pushed its Keychain records.
//
// Responsibility: keep a secret under a name and give it back. It knows
// nothing of providers or accounts; provider and integration share one Store
// and decide what the names mean. MemoryKeys is also llm's credstore.Store, so
// the ChatGPT sign-in record and host id live beside the keys under two
// reserved names.
//
// Invariants the types do not hold:
//   - MemoryKeys never writes a value anywhere. Its OnChange hears a name and
//     a deleted flag, never a value, and its errors name a credential, never
//     hold one.
//   - A name is a lower-case word of letters, digits and dashes. It is also
//     the file name in FileKeys, so it can never name a path.
//   - Export is the one call that hands a value out of MemoryKeys, and the
//     rpc layer is its only caller.
//
// See docs/architecture.md#2-responsibilities
package keystore
