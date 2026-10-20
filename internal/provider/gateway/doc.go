// Package gateway is how hearth talks to a service it reaches by address: the
// table of services it knows the address of, the check on an address, the
// list of models the service offers, and the check that a completion is one.
//
// Responsibility: the network side of an OpenAI-compatible service, with no
// state. It stores nothing and holds no key; the caller passes the key in
// and the client to send through. provider keeps the services the person
// added and builds their providers.
//
// Invariants the types do not hold:
//   - A key is never sent to another machine over plain http: CheckBaseURL
//     refuses that address, and ListModels expects an address it accepted.
//   - Every refusal is in plain words and never holds the key or what the
//     service sent. An address refusal wraps ErrAddress; a list that could
//     not be read wraps ErrList.
//   - A completion that ends with nothing said, from a body with no choices
//     in it, is an error naming the service and quoting the start of the
//     body with secrets removed (Check, over a client from Peeking).
//
// See docs/model.md#providers-and-models
package gateway
