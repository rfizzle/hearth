// Package office reads the text out of Word and PowerPoint files, and turns
// rows into a Markdown table, over the standard library alone.
//
// Responsibility: open an Office Open XML file (a zip) from a file, say which
// of Word, PowerPoint or Excel it is from its [Content_Types].xml part, walk
// the XML of a document or of one slide into text, and write rows as a
// Markdown table. It is handed files the owner did not write, and it sits in
// the trust boundary of decision D41.
//
// Invariants the types do not hold:
//   - A zip is never extracted. It is read through an io.ReaderAt and each
//     part is streamed. An entry's name is only ever a key into the archive's
//     own list, never a path to open, join onto a folder or log, so a name
//     such as "../../x" has no target; a relationship target from a part is
//     cleaned and looked up the same way. A name that occurs twice is
//     refused (ErrDamaged), so two readers cannot see two different parts.
//   - The sizes a zip header claims are not believed. A part whose header
//     claims more than Limits.Entry is refused before it is opened, and every
//     part is read through a meter that stops at Limits.Entry bytes of
//     decompressed data, whatever its header says; all the parts read from
//     one archive share Limits.Total. A file of more than Limits.Entries
//     entries is refused. All of these are ErrLimit.
//   - XML is streamed with encoding/xml, which has no DTD or entity
//     expansion. The walk stops with ErrLimit past Limits.Depth levels of
//     nesting or Limits.Elements elements in one part, and with the
//     context's error once it ends, checked before each part is opened and
//     every 64 tokens within one.
//   - The walkers recover every panic into ErrPanic; none leaves the
//     package. Nothing from a document is logged here.
//   - Text is returned as the document wrote it. Cleaning it for a model is
//     the caller's job (document.clean); only a table cell has its pipe and
//     line breaks changed, so a row stays one line.
//   - The walkers know the vocabulary by local element name and ignore the
//     namespace, so the strict and the transitional forms of a file read the
//     same.
//
// It imports the standard library only.
package office
