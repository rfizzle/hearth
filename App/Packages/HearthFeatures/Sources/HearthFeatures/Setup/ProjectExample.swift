// ProjectExample is one of the few projects Hearth shows an owner who has
// none, each a symbol, a title, one line and an opening paragraph. The list is
// fixed and ships in the app. A card only fills the project script's opener
// with the paragraph; the Chief of Staff drafts from it and the script runs as
// it always does.
// See docs/model.md#owner-interaction

import Foundation

/// An example project that opens the project setup with its paragraph filled.
public struct ProjectExample: Identifiable, Equatable, Sendable {
    public let id: String
    public let symbol: String
    public let title: String
    public let line: String
    /// What the opener holds when the example starts the setup.
    public let paragraph: String

    /// The four examples, in the order the sidebar shows them.
    public static let all: [ProjectExample] = [
        ProjectExample(
            id: "campaign", symbol: "megaphone", title: "Run a marketing campaign",
            line: "Who it is for, the message and the channels.",
            paragraph: "We want more small agencies to try our product this quarter. Help me "
                + "plan a campaign: who it is for, what we say, which channels we use and a "
                + "six-week schedule."),
        ProjectExample(
            id: "finance-report", symbol: "chart.bar.doc.horizontal",
            title: "Write the monthly finance report",
            line: "Revenue, spend and runway on one page.",
            paragraph: "I need our finance report for last month. Pull together revenue, spend "
                + "by team and how many months of runway we have, and point out anything that "
                + "moved a lot."),
        ProjectExample(
            id: "market", symbol: "magnifyingglass", title: "Research a market",
            line: "How big it is, who sells there and what they charge.",
            paragraph: "We are thinking of selling to law firms. Find out how big the market "
                + "is, who we would compete with and what they charge, and tell me if it is "
                + "worth a closer look."),
        ProjectExample(
            id: "launch", symbol: "shippingbox", title: "Launch a product update",
            line: "A date, the release notes and a note to customers.",
            paragraph: "We are launching our new API tier next month. Help me set the date, "
                + "write the release notes and the email to customers, and list what has to "
                + "be ready first."),
    ]
}
