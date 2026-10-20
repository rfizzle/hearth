// Package ask is the tool a bot asks its owner a question with: ask_owner,
// in plain words, with suggested answers, or with one control from the
// widget catalogue.
//
// Responsibility: the definition the model reads, the check that refuses a
// malformed request before it is put to the owner, and the words the owner
// sees for it.
//
// Invariants the types do not hold:
//   - Every valid call waits for the owner: the tool has no standing, so the
//     dispatcher gates it, and the answer reaches the bot from the
//     orchestrator, never from this executor. The executor only ever
//     returns a refusal.
//   - A request whose control the catalogue refuses never waits: the
//     dispatcher runs the check first and the model reads the reason.
//
// It imports tools, org and org/widget.
// See docs/model.md#owner-interaction.
package ask
