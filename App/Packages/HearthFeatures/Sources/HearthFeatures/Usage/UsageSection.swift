// UsageSection is the inspector's third section: what this bot cost this
// month as a UsageSummary (the cost, the budget and its Change…, Paused with
// Resume), and at its foot one line for the whole team's month. Before the
// first summary comes back the section is only its title. A failed call shows
// its text under the rows in the negative tone.
// See docs/model.md#usage

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Usage section.
struct UsageSection: View {
    let store: UsageStore
    let botID: String
    let botName: String

    var body: some View {
        InspectorSection(title: "Usage", detail: detail) {
            if let summary = store.current {
                UsageSummary(
                    subject: botName, totals: botTotals,
                    budgetCents: store.usage(of: botID)?.budgetCents ?? 0,
                    isPaused: store.isPaused(botID),
                    onChangeBudget: { cents in
                        Task { await store.setBudget(botID: botID, cents: cents) }
                    },
                    onResume: { Task { await resume(summary) } })
                Text(teamLine(summary))
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .monospacedDigit()
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.negative))
            }
        }
    }

    private var detail: String {
        store.currentMonth.map { UsageStore.title(month: $0) } ?? ""
    }

    /// The bot's totals; a bot with no usage yet has spent nothing.
    private var botTotals: Hearth_V1_UsageTotals {
        store.usage(of: botID)?.totals ?? .with { $0.priced = true }
    }

    /// Resumes the bot, and the whole team too when its budget paused it.
    private func resume(_ summary: Hearth_V1_SummaryResponse) async {
        if store.usage(of: botID)?.paused == true { await store.resume(botID: botID) }
        if summary.orgPaused { await store.resume(botID: "") }
    }

    /// "The whole team: $21.30 of $50 budget", or just the cost with no budget.
    private func teamLine(_ summary: Hearth_V1_SummaryResponse) -> String {
        let spent = UsageSummary.dollars(micros: summary.org.priced ? summary.org.costMicros : 0)
        guard summary.orgBudgetCents > 0 else { return "The whole team: \(spent)" }
        return "The whole team: \(spent) of \(UsageSummary.dollars(cents: summary.orgBudgetCents))"
    }
}
