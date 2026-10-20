// Package template holds the templates of shaped deliverables: the five Hearth
// ships, embedded, and the owner's, read from the Templates folder of the
// Hearth folder.
//
// Responsibility: a template is a spec with a name, a sentence on when to use
// it, and per section the widget kinds it takes. Parse reads one file, Load
// gathers the shipped five and the owner's, and Apply turns the sections a bot
// filled into a spec that has the template's kind, headings and columns.
//
// Invariants the types do not hold:
//   - The package imports the standard library and deliverable only. It reads
//     files in Load and nowhere else, and writes none.
//   - Load reads one folder: no recursion, no link followed, at most fifty
//     files of at most deliverable.MaxBytes. A bad file becomes a Problem and
//     never stops the rest; an owner's id that is a shipped id is a Problem.
//   - The folder is reserved from every bot. A bot names a template by id and
//     never reads or writes one.
//   - Sections are given to Apply by position, and the template fixes their
//     headings and columns. Every refusal wraps deliverable.ErrInvalid.
//
// See docs/model.md#work.
package template
