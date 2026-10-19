// Package files is the two file tools a bot works with: files.read and
// files.write, inside the folders its grants name.
//
// Responsibility: read a text file a page at a time, and create or replace a
// file, creating the folders above it, and hand the written file back for the
// owner to open.
//
// Invariants the types do not hold:
//   - An executor here runs only behind the dispatcher in the tools package,
//     which has resolved the path inside a granted folder. These executors
//     read and write that resolved path, never the argument the model sent.
//   - A write checks again after creating the folders above the file, and
//     refuses to follow a symlink at the file itself, so a link that appeared
//     after the dispatcher's check cannot carry the write out of the folder.
//
// It imports tools, org and workspace.
package files
