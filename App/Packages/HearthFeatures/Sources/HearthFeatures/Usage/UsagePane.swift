// UsagePane is Settings' Usage tab: a month picker over the months with any
// usage, then one card for the whole team and one for each bot, each the same
// UsageSummary the inspector shows. This month a card can change its budget
// and resume; a past month is a record and offers neither. Months and
// figures are the store's; the pane decides nothing.
// See docs/model.md#usage and docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Usage pane of Settings.
public struct UsagePane: View {
    let usage: UsageStore
    let bots: BotListStore

    /// A pane over `usage`, with a card for each of `bots`.
    public init(usage: UsageStore, bots: BotListStore) {
        self.usage = usage
        self.bots = bots
    }

    private var month: String {
        usage.selectedMonth ?? usage.currentMonth ?? ""
    }

    /// Whether the month shown is this one, whose budgets can change.
    private var isCurrent: Bool {
        month == usage.currentMonth
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            HStack {
                Text("Usage").font(Theme.title)
                Spacer()
                Picker("Month", selection: monthBinding) {
                    ForEach(usage.months, id: \.self) { month in
                        Text(UsageStore.title(month: month)).tag(month)
                    }
                }
                .labelsHidden()
                .fixedSize()
                .accessibilityLabel("Month")
                .disabled(usage.months.count < 2)
            }
            if let summary = usage.summary(month: month) {
                cards(summary)
            } else if usage.error == nil {
                ProgressView().controlSize(.small)
            }
            if let error = usage.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.negative))
            }
            Text(
                "Costs are estimates from published prices, not a bill. Work paid for by a plan is counted in tokens."
            )
            .font(Theme.caption)
            .foregroundStyle(Theme.color(.textSecondary))
        }
        .task { await usage.load() }
    }

    private var monthBinding: Binding<String> {
        Binding(
            get: { month },
            set: { next in Task { await usage.select(month: next) } })
    }

    @ViewBuilder
    private func cards(_ summary: Hearth_V1_SummaryResponse) -> some View {
        CardSurface {
            UsageSummary(
                name: "The whole team", subject: "The team", totals: summary.org,
                budgetCents: summary.orgBudgetCents, isPaused: summary.orgPaused,
                onChangeBudget: changeBudget(botID: nil), onResume: resume(botID: ""))
        }
        ForEach(bots.bots, id: \.id) { bot in
            let figures = summary.bots.first { $0.botID == bot.id }
            CardSurface {
                UsageSummary(
                    name: bot.name, subject: bot.name,
                    totals: figures?.totals ?? .with { $0.priced = true },
                    budgetCents: figures?.budgetCents ?? 0, isPaused: figures?.paused ?? false,
                    onChangeBudget: changeBudget(botID: bot.id), onResume: resume(botID: bot.id))
            }
        }
    }

    /// Change… for a card, this month only; the team's has no bot id.
    private func changeBudget(botID: String?) -> ((Int64) -> Void)? {
        guard isCurrent else { return nil }
        return { cents in
            Task {
                if let botID {
                    await usage.setBudget(botID: botID, cents: cents)
                } else {
                    await usage.setOrgBudget(cents: cents)
                }
            }
        }
    }

    /// Resume for a card, this month only; an empty id is the team.
    private func resume(botID: String) -> (() -> Void)? {
        guard isCurrent else { return nil }
        return { Task { await usage.resume(botID: botID) } }
    }
}
