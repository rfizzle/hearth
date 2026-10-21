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
            id: "trip", symbol: "airplane", title: "Plan a trip",
            line: "Where to go, what to book and when.",
            paragraph: "I want to plan a trip. Help me pick a place, a rough budget and dates, "
                + "and list what I need to book."),
        ProjectExample(
            id: "newsletter", symbol: "envelope", title: "Launch a newsletter",
            line: "A first issue, out to your readers.",
            paragraph: "I want to start a newsletter. Help me choose what it is about, "
                + "draft the first issue and get it sent."),
        ProjectExample(
            id: "finances", symbol: "chart.pie", title: "Tidy the finances",
            line: "Bills, subscriptions and records in order.",
            paragraph: "I want to get my finances in order. Go through my bills and "
                + "subscriptions, and set up a simple record of what I owe and what I pay."),
        ProjectExample(
            id: "skill", symbol: "book", title: "Learn a skill",
            line: "A plan and a few weeks of practice.",
            paragraph: "I want to learn a new skill. Help me choose a first goal, make a "
                + "weekly practice plan and keep track of how it is going."),
    ]
}
