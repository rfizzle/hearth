// Package handbook is the pages the owner writes for every bot, or for one:
// which a bot reads, whether a page still fits, and the block that goes into
// its prompt.
//
// Responsibility: list, put and delete pages over a store seam, refuse a page
// that would take a reader's view past the cap, and render the pages a run
// carries. Selection and the byte count are memory's pure rules.
//
// Invariants the types do not hold:
//   - The cap is bytes of Markdown, not tokens: the core has no tokenizer.
//   - Render leaves a page out whole, never cuts one, and says which it left.
//
// See docs/model.md#knowledge.
package handbook
