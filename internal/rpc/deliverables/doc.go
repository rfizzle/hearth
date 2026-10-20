// Package deliverables serves what a shaped deliverable is made of: the
// MeasureService, the office's figures read by id, and the
// DeliverableService, the templates a bot fills and the owner keeps.
//
// Responsibility: translate between proto messages and the measure and
// template packages, and keep the owner's template files in the Templates
// folder of the Hearth folder: save one after the template package accepts
// it, remove one, and list the shipped five with the owner's. Nothing else;
// what an id means and what a template allows are those packages' rules.
//
// Invariants the types do not hold:
//   - Only a file named <id>.deliverable.json directly in the Templates
//     folder is written or removed, and the path is resolved through
//     workspace so a link cannot carry it outside the Hearth folder. The
//     file is opened without following a link.
//   - A shipped template is never replaced or removed, and a bot has no
//     call here: the folder is reserved from every tool and grant.
//   - A refusal in words (an id, a template) is invalid_argument with the
//     words; a failed source is internal.
//   - Without a Hearth folder only the shipped templates are listed and a
//     save is a failed precondition.
//
// Seams it is handed (Deps): the measure sources and the function that says
// where the Hearth folder is now. See docs/model.md#work.
package deliverables
