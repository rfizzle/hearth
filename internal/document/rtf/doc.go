// Package rtf strips an RTF file to its text.
//
// Responsibility: walk the file's groups and control words and return the
// words a reader would see, with paragraphs as newlines, tabs, the accented
// letters written as \'hh (read as Windows-1252) and the characters written
// as \uN. Formatting is dropped; this is not a converter.
//
// Invariants the types do not hold:
//   - The file is untrusted. Groups nest at most MaxDepth deep, a control
//     word's number has at most ten digits, a \bin run is skipped within the
//     file and never allocated, and the text returned is at most the limit
//     the caller gives. Each of these is a refusal or a stop, never a panic.
//   - Groups that hold no reading text are dropped whole: the font, colour,
//     style, list and revision tables, the document information, pictures,
//     embedded objects, field instructions and any group that begins {\*
//     (a destination a reader may not know).
//   - The walk is one pass over a reader with no recursion, so the depth cap
//     bounds a stack of small structs, not the call stack. The context is
//     checked every 64 KB read.
//   - Nothing from the file is written to a log.
//
// It imports the standard library only.
package rtf
