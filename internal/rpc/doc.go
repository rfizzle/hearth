// Package rpc serves the hearth.v1 contract to the app over HTTP.
//
// Responsibility: take Options, check the launch token on every request,
// and register the services: staff serves the org, the bots with their
// templates and grants, the conversations with their newest run and a retry,
// the Inbox of decisions and answers and the event stream, beside the Ping
// and StreamHello of the spike; accounts serves the model accounts and the
// integrations; books serves the calls about notes, a bot's settings, the
// handbook, usage and the tiers; deliverables serves the office's figures by
// id and the templates of shaped deliverables. Each translates between proto messages and
// the domain values behind them. Nothing else.
//
// Invariants the types do not hold:
//   - Handlers hold no domain logic, in the packages that serve them: see
//     staff, accounts and books for what a handler may decide.
//   - The token is compared in constant time, ahead of the router, so a
//     request without it reaches no handler, registered or not.
//   - A service is served only when its dependencies are set.
//   - The stream interval is an option, so tests stream without sleeping.
//
// rpc imports ten internal packages, the warning ceiling, and that is
// accepted: the next service splits the registration into its own package
// rather than adding an eleventh.
//
// Seams it is handed (Options): the version string Ping reports, the word
// list StreamHello cycles through and the interval between words; the store,
// the orchestrator, the accounts, the integrations, the role templates, the
// function that drafts a bot, the function that creates the Hearth folder,
// the event bus, the handbook's pages, the meter and the sources of the
// measures with the place of the Hearth folder. See
// docs/architecture.md#1-layering
package rpc
