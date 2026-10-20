// Package native is the core's client of the app's native service: the
// second loopback listener the app serves while it is open, for what only
// Apple's frameworks do. Today that is reading the text of picture pages and
// images.
//
// Responsibility: call the app's NativeService with the generated Connect
// client in its JSON form, with the app's own token for this direction as the
// bearer (never the launch token, which controls the helper), and turn the
// answer into what document asks of a Recogniser. tools/files and document
// never import the generated code; the client is handed to the orchestrator,
// not to the contract handlers.
//
// Invariants the types do not hold:
//   - The address is a loopback IP and a port, checked when the client is
//     made, and no proxy is ever used, so the token never leaves the machine.
//   - Every call is bounded: 2 s to connect, 30 s for the whole call, at
//     most ten pages, and a reply over 4 MB is refused unread.
//   - The app is a separate process: what it answers is data. Only the pages
//     asked for are passed on, at most five labels each, and document cleans
//     and caps the text as it does any page's.
//   - A call that cannot connect, or that the app answers as busy, marks the
//     client down and returns ErrNoApp without asking again. ForRun hands each run a client of its own, so the
//     mark lasts one run and the next run tries the app again.
//   - Nothing the app answers is logged.
//
// Seams it is handed: the address from hearthd serve's --native flag and the
// native token the app sent on stdin after the launch token.
// See docs/decisions.md#hearth--decisions
//
// It imports document, the generated contract, connect and the standard
// library.
package native
