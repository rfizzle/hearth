// Package books serves what the org keeps on its bots: the owner's edits to
// a bot and the notes it keeps, the handbook pages every bot reads, what the
// bots cost against their budgets, and which model each quality tier asks
// for.
//
// Responsibility: the HandbookService and UsageService handlers, and the
// BotService and ProviderService calls about notes, a bot's settings, the
// tiers and the models on offer, which rpc's own services take on. Each
// handler translates the request, asks the package that owns the rule, and
// translates the answer; nothing else.
//
// Invariants the types do not hold:
//   - Handlers hold no domain logic. The note store keeps the notes, the
//     handbook package refuses a page that overfills a reader, the meter
//     owns the budgets and the pause, and the models package says what is on
//     offer. What a handler decides is the contract's own: an empty field in
//     an edit keeps what is stored, a missing id is invalid_argument, and a
//     title is words a person typed.
//   - Every error picks its code in wire.Fail.
//   - No reply and no event carries a credential: the accounts are read for
//     the models they offer, never for a key.
//   - A change is told to the bus after it is stored, never before.
//
// Seams it is handed (Deps): the store, the model accounts, the handbook's
// pages, the meter, and the function that publishes to the bus.
// See docs/model.md#knowledge and docs/architecture.md#1-layering
package books
