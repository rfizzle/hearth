// Package sheet reads an Excel workbook as Markdown tables, one per sheet.
//
// Responsibility: open an .xlsx from bytes in memory, read each sheet's rows
// with the numbers and dates shown as Excel shows them, and return a
// Markdown table per sheet within fixed caps. The notices and the words
// around a table belong to the document package; this package returns
// numbers and names.
//
// Invariants the types do not hold:
//   - The file is untrusted. Before the library sees it, the zip is walked
//     here: at most MaxEntries entries, no entry whose header claims more
//     than MaxEntry bytes, no more than MaxTotal bytes in all, and every
//     entry is decompressed once against those numbers whatever its header
//     says, so a header that lies is refused. The library is then given the
//     same numbers as its limits, with the in-memory threshold raised to
//     match and its temporary folder pointed at a path that is not a folder,
//     so nothing a workbook holds is ever written to disk.
//   - A read is bounded: at most MaxSheets sheets, MaxCells cells shown per
//     sheet (rows times the columns in use) and MaxCellRunes characters in a
//     cell. The deadline is checked between rows and between sheets, and a
//     sheet the deadline reaches is dropped whole.
//   - The library is third-party code on a file the owner did not write. A
//     panic in it is contained: in a sheet it leaves that sheet unread, in
//     opening it is ErrDamaged. Two reads run at once at most; a read that
//     never returns keeps its place until the helper exits. What this
//     package cannot bound is the library's own memory on a crafted file
//     inside the byte caps, the residual decision D41 accepts.
//   - Nothing from the workbook's content is written to a log.
//
// See docs/decisions.md#hearth--decisions
//
// The table writer and the cell cap are the office package's, shared with the
// CSV reader. It imports office, excelize and the standard library.
package sheet
