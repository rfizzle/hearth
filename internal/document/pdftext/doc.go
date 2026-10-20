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
//   - The parser can also loop for ever: on a trailer whose Prev points at
//     itself, or a page tree that points back at itself. Open and Page run
//     the parse on its own goroutine and give up when their context ends;
//     the abandoned goroutine keeps running until the helper exits, so a
//     caller stops using the Reader once a call returns the context's error.
//   - At most two parses run or are stuck at a time, in the whole process.
//     A parse holds its slot until it returns, so a crafted file can pin two
//     cores and no more however often it is read; a call that finds no slot
//     free before its context ends returns the context's error without
//     parsing.
//   - The parser decompresses a page's content in memory without a cap. A
//     page that is large enough exhausts memory and ends the process; the
//     caller's caps on bytes and pages bound the file, not that.
//   - A PDF encrypted with an empty password opens; one with a password is
//     ErrPassword. No password is ever tried.
//
// It imports the parser and the standard library only.
package pdftext
