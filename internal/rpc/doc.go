// Package rpc serves the hearth.v1 contract to the app over HTTP.
//
// Responsibility: register the generated Connect handlers, check the launch
// token on every request, and translate between proto messages and the
// domain values behind them. It serves the org, the bots, their templates
// and their grants, the conversations with their newest run and a retry, the
// Inbox of decisions and answers, the model accounts, the integrations and
// the event stream, beside the Ping and StreamHello of the spike. The calls
// about notes, a bot's settings, the handbook, usage and the tiers are
// books', registered here. Nothing else.
//
// Invariants the types do not hold:
//   - Handlers hold no domain logic. A handler with an if about the domain
//     belongs in the package that owns that rule; what a handler may decide
//     is a code: a store.ErrNotFound is not_found, a run in progress is
//     failed_precondition, a folder another bot has is already_exists, an
//     empty required field is invalid_argument. Every domain error picks its
//     code in one function, wire.Fail, never in a handler; wire holds the
//     translation more than one service needs.
//   - A handler never widens access: a grant is stored as the owner set it,
//     after the inbox package has refused a mode its capability never has,
//     and a decision on an item goes to the orchestrator, which refuses one
//     already made.
//   - A missing org is an empty answer, not an error: onboarding has not
//     happened.
//   - The token is compared in constant time, ahead of the router, so a
//     request without it reaches no handler, registered or not.
//   - An account carries a label and never a key; the integration's label is
//     the last four characters.
//   - A service is served only when its dependencies are set.
//   - The stream interval is an option, so tests stream without sleeping.
//
// Seams it is handed (Options): the version string Ping reports, the word
// list StreamHello cycles through and the interval between words; the store,
// the orchestrator, the accounts, the integrations, the role templates, the
// function that drafts a bot, the function that creates the Hearth folder,
// the event bus, the handbook's pages and the meter. See
// docs/architecture.md#1-layering
package rpc
