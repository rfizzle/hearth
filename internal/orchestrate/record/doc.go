// Package record is what one run writes as it goes: its messages, its steps
// and how it ended, stored and published as they happen.
//
// Responsibility: be the agent's sink for one run. The bot's reply is one
// text message per stretch of text, a tool call closes the stretch before
// it, and the run ends done, cancelled, waiting on the owner or failed with
// the reason in the thread; a call the run parked on becomes the request in
// the thread and the item in the Inbox, and a provider failure the item the
// owner retries or acknowledges.
//
// Invariants the types do not hold:
//   - The service is reached only through a Host. The conversation is freed
//     through Host.Release once the final state is stored and before the
//     bus hears the run ended, so a message sent on that news is not refused
//     as busy; a waiting run's conversation is held by its open item
//     instead.
//   - A write that fails is kept, not returned: the first one fails the run
//     when it finishes. Writes use a context a stop does not cancel, so what
//     a stopped run wrote is still saved.
//   - A panic is recorded once, logged with its stack, and stops the turn.
//
// See docs/model.md#9-failure-handling and docs/model.md#8-approvals.
package record
