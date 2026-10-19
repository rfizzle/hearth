// Package web is the two web tools a bot works with: web.fetch, which reads
// a public page as text, and web.search, which asks a search service.
//
// Responsibility: fetch a page under the address policy and reduce it to
// text within a cap; define the seam a search service sits behind and the
// tool over it; and implement Brave Search as the first service behind the
// seam. Another service is one more file here beside brave.go.
//
// Invariants the types do not hold:
//   - Every hop of a fetch, the first and each redirect, has its host looked
//     up and every address checked before a request is made, and the
//     connection goes to the checked address and no other. A request that
//     reaches the dial without a checked address is refused there.
//   - The address policy guards web.fetch only. A search service's address
//     is fixed in its file, so a query never chooses where a request goes.
//   - web.search exists only when a Searcher is configured; with none, the
//     web.fetch description tells the bot to start from sites it knows.
//
// Seams it is handed: a Searcher, and in tests a resolver and an HTTP
// transport in place of the network. It imports tools and org.
package web
