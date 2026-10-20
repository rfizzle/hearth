// Package propose is the tool the Chief of Staff proposes a project to the
// owner with: propose_project, five strings that become a card the owner
// confirms or turns down.
//
// Responsibility: the definition the model reads, the check that bounds the
// model's text before anything is stored, and the words the owner sees for
// the call. Storing the proposal is the orchestrator's, handed in as a
// Proposer, so this package does not import the store.
//
// Invariants the types do not hold:
//   - The model's text is bounded before the proposer runs: the name is one
//     plain line of at most 80 characters, each of the four brief parts is
//     non-empty and at most 4,000. A refusal says what to change, and the
//     proposer is not called.
//   - The text is passed on as written. No folder name is made from it here;
//     that happens only when the owner confirms.
//   - A proposal assigns nothing and starts nothing, so the tool is
//     read-tier; the dispatcher still offers it only to a bot holding the
//     project.propose capability.
//
// It imports tools, org/memory and project.
// See docs/model.md#5-the-chief-of-staffs-routing-policy.
package propose
