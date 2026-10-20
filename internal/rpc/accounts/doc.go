// Package accounts serves ProviderService and IntegrationService: which
// model accounts are signed in, their credentials, and the optional services
// that are not model providers.
//
// Responsibility: translate between those two services' messages and the
// provider and integration packages behind them, and name the bus bodies a
// change to an account publishes. Nothing else.
//
// Invariants the types do not hold:
//   - An account carries a label and never a key; the integration's label is
//     the last four characters. The one reply that carries a credential value
//     outside a sync is ExportCredential, and no event carries one.
//   - Handlers hold no domain logic; wire.Fail picks every domain error's
//     code.
//   - The change hooks reach the bus through Register, so whoever changed an
//     account, a sign-in finishing in the background included, is heard.
//
// Seams it is handed (Deps): the accounts, the integrations, the deps books
// gives the tiers and the models, and the function that publishes to the
// bus. See docs/architecture.md#1-layering
package accounts
