// Package rpc serves the hearth.v1 contract to the app over HTTP.
//
// Responsibility: register the generated Connect handlers, check the launch
// token on every request, and translate between proto messages and the
// values the handlers return. Nothing else.
//
// Invariants the types do not hold:
//   - Handlers hold no domain logic. A handler with an if about the domain
//     belongs in the package that owns that rule.
//   - The token is compared in constant time, ahead of the router, so a
//     request without it reaches no handler, registered or not.
//   - The stream interval is an option, so tests stream without sleeping.
//
// Seams it is handed (Options): the version string Ping reports, the word
// list StreamHello cycles through, and the interval between words.
package rpc
