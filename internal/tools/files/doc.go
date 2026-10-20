// Package files is the three file tools a bot works with: files.read,
// files.write and files.delete, inside the folders its grants name.
//
// Responsibility: read a text file a page at a time; create or replace a
// file, creating the folders above it, and hand the written file back for the
// owner to open; and delete one file once the owner has allowed it.
//
// Invariants the types do not hold:
//   - An executor here runs only behind the dispatcher in the tools package,
//     which has resolved the path inside a granted folder. These executors
//     read, write and delete that resolved path, never the argument the
//     model sent.
//   - A write checks again after creating the folders above the file, and
//     refuses to follow a symlink at the file itself, so a link that appeared
//     after the dispatcher's check cannot carry the write out of the folder.
//   - A delete removes one regular file and nothing else: never a folder,
//     never a path that ends in a link, and only the file the call's own
//     path still names, reached through the granted folder as a root. It is
//     deleted, not moved to the Trash.
//
// It imports tools, org and workspace.
package files
