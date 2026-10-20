// Package document reads the text out of a file a bot is pointed at: a PDF,
// a Word file, a PowerPoint deck, a CSV, an Excel workbook, an RTF file, or
// the kind and size of a picture.
//
// Responsibility: decide what kind of file it is from its name and its first
// bytes, read it within fixed caps, and return its text with page markers
// and the notices that tell the model what was left out. It sits below the
// tools, so a tool, the context assembly or a later service can read a
// document without importing the tool layer; the engines are decision D38.
//
// Invariants the types do not hold:
//   - A parsed document is data, never instructions. The text is cleaned
//     before it leaves this package: invalid UTF-8 and control characters
//     other than newline and tab are gone, and a line that begins as the
//     app's own voice does is pushed off the start of the line. Every notice
//     is one of a fixed set of strings in brackets; only a file name and
//     numbers are filled in. The limits and this rule are decision D41.
//   - A read is bounded: a file over 64 MB is refused before it is opened as
//     a document, a call reads at most 100 pages and 2 MB of text, and the
//     whole read stops after a minute and says how far it got. The deadline
//     is checked between pages and while a page is parsed.
//   - A reader runs a third-party parser on a file the owner did not write.
//     A page the parser fails on or panics on is reported as unread; it
//     never ends the caller. What this package cannot bound is the parser's
//     own memory use on a crafted file; see pdftext.
//   - Nothing from a document's content is written to a log, here or by a
//     caller of this package.
//   - A picture page (a scan, or a page whose text layer is garbage) is
//     found by a fixed rule and offered to the Recogniser, the app while it
//     is open, at most ten pages a call and within the read's minute. What
//     it answers is text from outside this process: only the pages asked
//     for are kept, and their text is cleaned and capped as any page's is.
//     A page it does not answer, or every page when there is no app, is
//     reported as a picture that could not be read.
//
// A kind is one row in kinds.go: its name, extensions, the check its first
// bytes must pass, and its reader. A new kind is a new row and a sub-package. A Word or PowerPoint file is a
// zip read by office under limits on entries, bytes and XML depth; a CSV is
// read here into a Markdown table of at most 10,000 cells. A workbook is read
// by sheet, in memory and under its own zip limits, up to 20 sheets of 10,000
// cells, or the one sheet the call names; an RTF file by rtf, with a depth and
// a size cap. A picture is described from its header alone (format and
// pixels, never decoded) and one over 50 megapixels is refused; its text and
// what it shows are asked of the Recogniser, and the notice says whether the
// app was open.
//
// See docs/decisions.md#hearth--decisions
//
// It imports pdftext, office, sheet, rtf, picture, x/image/webp and the standard
// library.
package document
