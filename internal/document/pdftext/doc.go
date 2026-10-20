// Package pdftext reads the text layer of a PDF page by page, over a
// third-party parser that is handed untrusted files.
//
// Responsibility: open a PDF from a file, say how many pages it claims, and
// return one page's text. Nothing else of the parser reaches the caller.
//
// Invariants the types do not hold:
//   - The parser can panic on a crafted file, so every call into it sits
//     behind a recover: Open turns a panic into ErrDamaged and Page into
//     ErrPage. A panic never leaves this package.
//   - The parser can also loop for ever on a page tree that points back at
//     itself. Page runs the parse in its own goroutine and gives up when its
//     context ends; the abandoned goroutine keeps running until the helper
//     exits, so a caller stops using the Reader once a Page returns the
//     context's error.
//   - The parser decompresses a page's content in memory without a cap. A
//     page that is large enough exhausts memory and ends the process; the
//     caller's caps on bytes and pages bound the file, not that.
//   - A PDF encrypted with an empty password opens; one with a password is
//     ErrPassword. No password is ever tried.
//
// It imports the parser and the standard library only.
package pdftext
