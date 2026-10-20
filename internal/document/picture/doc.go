// Package picture is the picture page: the fixed rule that finds a page whose
// text is not text, the offer of such pages to a Recogniser (the app, while it
// is open) in batches, and the notices that say how each was read or why not.
//
// Invariants the types do not hold:
//   - What a Recogniser answers comes from outside the process. Offer keeps
//     only the pages it asked for, each once; cleaning and capping the text is
//     the caller's, document's, as for any page.
//   - A batch is at most Batch pages; once the app is found gone, or the
//     read's time is up, no further batch is sent.
//   - Every notice is a fixed sentence in brackets; only a file name, which
//     the caller has made one line, and page numbers are filled in.
//
// See docs/decisions.md#hearth--decisions
package picture
