// Package workspace owns the Hearth folder on disk: its layout, and the one
// check that turns a path a model asked for into a path inside it.
//
// Responsibility: create the folder and its Inbox, name a bot's own folder,
// and resolve a requested path against a root, refusing any result outside
// the root.
//
// Invariants the types do not hold:
//   - Resolve is the only place a path from a model becomes a path on disk.
//     It follows every symlink that exists, including a dangling one, before
//     it compares, so a link inside the folder cannot lead out of it.
//   - The answer holds at the moment of the call. A caller that writes
//     refuses to follow a symlink created after it, rather than trusting the
//     earlier answer.
//   - A root is an absolute path; a relative one is refused, not resolved
//     against the working directory.
//
// It imports org only, for the form of a bot's name its folder takes.
package workspace
