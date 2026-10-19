// Package provider is hearth's one door to a model: it names the app to llm,
// signs a person in with their ChatGPT plan, and streams one reply from it.
//
// Responsibility: own every import of github.com/rfizzle/llm outside the agent
// loop. The helper's commands call this package and never llm directly.
//
// Invariants the types do not hold:
//   - Tokens live in a file store under the person's application support
//     folder, a 0700 directory of 0600 files, and nowhere else. The core never
//     prints a token, and a signed-out account is reported in words that name
//     the command to run, not the error chain.
//   - A returning launch reads the stored sign-in; it opens no browser and
//     asks for nothing.
//
// Seams it is handed: the http client that reaches the authorization server,
// the function that opens the browser, and the directory the store lives in.
//
// The file store is a stopgap. The planned follow-on is a Keychain-backed
// store behind the same interface, so the tokens leave the disk.
package provider
