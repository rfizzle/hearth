// Package deliverables is the tool a bot writes a shaped deliverable with:
// deliverables.write, a stem, an optional template and a spec, which becomes
// the spec, a web page and a PDF beside each other in a granted folder.
//
// Responsibility: the definition the model reads, built per run from the
// templates loaded for it and the catalogue of widgets; the check that runs
// before the call is placed (the spec parsed, fitted to its template,
// validated, and each measure id parsed, its kind matched to its widget and
// its aliases resolved for the caller); the fill of each bound widget from
// the office's own figures; the three files; and the words the owner sees.
//
// Invariants the types do not hold:
//
//   - The spec is model output. It is parsed and validated in full before
//     the call is placed, so a malformed one is a refusal the model reads
//     and never a card the owner sees. Nothing in it is executed, fetched or
//     opened, and nothing in it names a file.
//
//   - The output names derive from the call's resolved path, the stem the
//     dispatcher placed inside a granted folder, never from the spec: the
//     stem with .deliverable.json, .html and .pdf. Each is checked again
//     against the reserved folders and opened through files.OpenForWrite,
//     which refuses a link, a folder or a file with a second name.
//
//   - Every file is drawn before the first is written; they are written in
//     the order spec, page, PDF, and a failure after the first write says
//     which were written and which failed.
//
//   - A bound value is the core's: the measure layer reads it at write time
//     and overwrites whatever number the bot gave, and the time it was read
//     is filled in. The aliases this, me, org and current are resolved for
//     the calling bot and the conversation's project; a subject that does
//     not exist is a refusal.
//
//   - Until a deliverable can be drawn as a PDF, the PDF step returns
//     paper.ErrNotYet and the tool writes two files and says so.
//
// It imports tools, org, workspace, deliverable, deliverable/template,
// measure, render/html, render/paper and tools/files.
// See docs/decisions.md for D44, D46 and D47.
package deliverables
