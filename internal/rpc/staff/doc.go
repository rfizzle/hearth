// Package staff serves the services about the org and the people in it:
// OrgService, BotService (with grants), ConversationService (with retry),
// InboxService and EventService, beside the Ping and StreamHello of the
// spike.
//
// Responsibility: translate between those services' messages and the org,
// store and orchestrate values behind them, convert the bus bodies into the
// event stream's messages, and hand a body it does not know to accounts and
// books. The calls about notes and a bot's settings are books', embedded
// into BotService. Nothing else.
//
// Invariants the types do not hold:
//   - Handlers hold no domain logic. A handler with an if about the domain
//     belongs in the package that owns that rule; what a handler may decide
//     is a code: a run in progress is failed_precondition, a folder another
//     bot has is already_exists, an empty required field is
//     invalid_argument. Every domain error picks its code in wire.Fail.
//   - A handler never widens access: a grant is stored as the owner set it,
//     after the inbox package has refused a mode its capability never has,
//     and a decision on an item goes to the orchestrator, which refuses one
//     already made.
//   - A missing org is an empty answer, not an error: onboarding has not
//     happened.
//   - Every enum that differs in kind goes through a table, never a cast,
//     because the domain's numbers are stored on disk and the contract's are
//     appended to; a test holds each table to both lists.
//   - The stream interval is a dependency, so tests stream without sleeping.
//
// Seams it is handed (Deps): the version string Ping reports, the word list
// StreamHello cycles through and the interval between words; the store, the
// orchestrator, the role templates, the function that drafts a bot, the
// function that creates the Hearth folder, the event bus, the deps books
// takes, and the function that publishes to the bus.
// See docs/architecture.md#1-layering
package staff
