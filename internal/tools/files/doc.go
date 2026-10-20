// Package files is the four file tools a bot works with: files.read,
// files.write, files.render and files.delete, inside the folders its grants
// name.
//
// Responsibility: read a file as text a window at a time, a document (PDF,
// Word, PowerPoint, CSV, Excel, RTF or an image's description) through the
// document package, with the pages or sheet it was asked for and the
// notices it returned; create or replace a
// file, creating the folders above it, and hand the written file back for the
// owner to open; render a Markdown file as a PDF beside it through the paper
// package; and delete one file once the owner has allowed it.
//
// Invariants the types do not hold:
//
//   - An executor here runs only behind the dispatcher in the tools package,
//     which has resolved the path inside a granted folder. These executors
//     read, write and delete that resolved path, never the argument the
//     model sent.
//
//   - A write checks again after creating the folders above the file, and
//     refuses to follow a symlink at the file itself, so a link that appeared
//     after the dispatcher's check cannot carry the write out of the folder.
//     It refuses a file with a second name, a hard link, which the write
//     would change under that name too.
//
//   - A delete removes one regular file and nothing else: never a folder,
//     never a path that ends in a link, and only the file the call's own
//     path still names, reached through the granted folder as a root. It is
//     deleted, not moved to the Trash.
//
//   - files.render is checked against the write grant and reads its source
//     under that grant: a bot that may write in a folder may read the
//     Markdown it wrote there, and a folder granted write but not read is
//     an odd setup. It writes only the source's stem with .pdf beside it,
//     through the same opening as files.write, and reaches an image only
//     inside the granted folder and outside the reserved ones.
//
// A document is data: what the document package returns is the result, and
// nothing from its content is logged here.
//
// It imports tools, org, document, render/paper and workspace.
package files
