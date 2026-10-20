// Package docx writes a Markdown document as a Word file: the same parse the
// PDF is made from, set in Word's built-in styles so Word, Pages and Google
// Docs open and restyle it.
//
// Responsibility: one entry point, Write, that walks the tree of deliverable's
// strict Markdown reader into the parts of a .docx (the content types, the
// relationships, document.xml, styles.xml and numbering.xml, and the images)
// and zips them. Headings are Title and Heading1 to Heading6, paragraphs carry
// bold, italic, struck and code runs, lists are Word lists, a table is a
// TableGrid table with a repeating header row, a quote is Quote, a code block
// is a paragraph of Code per line, and an image is an inline picture.
//
// Invariants the types do not hold:
//   - A write is reproducible: no clock (every zip entry has the same fixed
//     time), no map order (the parts are written sorted), nothing from the
//     machine. The same source and options give the same bytes.
//   - The document is data. A link is text with its address after it in
//     brackets and never a hyperlink relationship, so nothing in the file
//     acts when it is opened; raw HTML is dropped with a notice; nothing from
//     the document names a style, a font or a part; and a file is reached only
//     through the Opener the caller hands it, for a relative path, at most 20
//     images of at most 20 megapixels, PNG or JPEG. An address with a scheme is
//     refused before anything is opened. A PNG is decoded and written again, so
//     the file Word sees was made here.
//   - A write is bounded: a source of at most 1 MB that deliverable's
//     CheckMarkdown accepts before the parser sees it, the parse on its own
//     goroutine so the call returns at the deadline of 60 seconds, a body of
//     at most 32 MB, 40 MB of images, a walk that stops nesting at a fixed
//     depth, and 1,000 numbered lists. A panic is ErrRender.
//   - Notices are fixed sentences naming only the file; nothing from the
//     document is logged.
//
// It parses with deliverable's Markdown reader and writes with the standard
// library alone. See docs/decisions.md for D37, D39 and D41.
package docx
