// Package grid draws a shaped deliverable as a PDF in the print style: the
// same sections, the same grid of cards and the same charts as its web page,
// on paper's Document.
//
// Responsibility: Render, which sets the title and subtitle, then each
// section's heading and its grid of columns with 12 pt gutters; a card per
// widget with a hairline edge, a callout's filled in its tone; a page break
// before a row of cards that does not fit, a card taller than a page drawn
// alone and continued, its edge drawn on each page it crosses, and a table's
// header row repeated on each page it continues on; a chart drawn from
// chart.Layout's primitives with fpdf's vector calls, scaled to its card.
//
// Invariants the types do not hold:
//   - A render is reproducible: no clock (the PDF's dates are ModTime, the
//     spec file's), no map, numbers through deliverable.Format, and a chart
//     drawn from the layout every renderer reads, never rasterised.
//   - The spec is treated as valid (deliverable.Validate has passed), and
//     still nothing in it is executed, opened or named as a font or a file:
//     text passes through paper's Clean, and the Markdown of a text or a
//     callout through deliverable.CheckMarkdown and paper's walk.
//   - A render is bounded: 60 pages, refused above in words; paper's
//     deadline over the parse and the layout; a panic in the drawing library
//     is paper's ErrRender. A row's height is measured by drawing it on a
//     scratch page first, so a render costs about twice its drawing.
//
// It imports deliverable, render/chart, render/fonts and render/paper, and
// draws with go-pdf/fpdf.
// See docs/decisions.md for D45 and D47.
package grid
