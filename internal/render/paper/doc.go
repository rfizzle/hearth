// Package paper draws a document as a PDF in hearth's print style: a
// Markdown file today, through Markdown.
//
// Responsibility: the page (Document: the paper, the margins, the embedded
// fonts, "1 of 3" in the footer, the page cap, the PDF's fixed producer and
// its dates) and the style table it draws with (Style, the print form of
// docs/design.md), and one entry point that walks a Markdown tree onto it:
// headings, paragraphs with bold, italic, struck and code text, links as
// text with their address, lists and task lists, quotes, code blocks,
// tables, rules and images. Another entry point can draw onto the same
// Document; Render, for a shaped deliverable, answers ErrNotYet until it
// does.
//
// Invariants the types do not hold:
//   - A render is reproducible: no clock (the PDF's dates are the source's
//     modification time), no map order, the producer fixed as Hearth.
//   - The document is data. Raw HTML is dropped with a notice; a link is
//     drawn as text and never becomes an action in the PDF; nothing from the
//     document names a font, and a file is reached only through the Opener
//     the caller hands it, for a relative path, at most 20 images of at most
//     20 megapixels, PNG or JPEG. An address with a scheme is refused before
//     anything is opened.
//   - A render is bounded: a source of at most 1 MB that deliverable's
//     CheckMarkdown accepts before the parser sees it, the parse on its own
//     goroutine so the call returns at the deadline; 200 pages, 60 seconds
//     and 40 MB of images; a walk that stops nesting at a fixed depth; and
//     fpdf given text a word at a time. A panic in the parser or the drawing
//     library is ErrRender.
//   - Notices are fixed sentences naming only the file; nothing from the
//     document is logged.
//
// It parses with deliverable's strict Markdown reader, the one every
// renderer shares, and draws with go-pdf/fpdf and the fonts package.
// See docs/decisions.md for D39 and D41.
package paper
