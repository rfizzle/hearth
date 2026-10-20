// UsageSummary is what a bot, or the whole team, cost in a month. The cost is
// the headline, in US dollars the way the system writes them; a month with no
// price for its models shows tokens as the headline instead, never $0. Under
// it the requests and steps in a caption, and the Tokens disclosure, closed,
// for anyone who wants the count. A budget is a quiet bar and one line, "$18.40
// of $20", in the neutral tone until the cost passes nine tenths of the limit,
// attention after, negative at it, with Change… opening the small BudgetField.
// A paused bot wears a Paused pill with Resume beside it. With a name it is a
// row of the Settings pane; without one it fills the inspector's section. It
// decides nothing: Change… and Resume call back.
// See docs/design.md#colour and docs/design.md#forms-sheets-and-settings

import HearthDesign
import Foundation
import HearthKit
import SwiftUI

/// One month's usage for a bot or for the team.
public struct UsageSummary: View {
    let name: String?
    let subject: String
    let totals: Hearth_V1_UsageTotals
    let budgetCents: Int64
    let isPaused: Bool
    let onChangeBudget: ((Int64) -> Void)?
    let onResume: (() -> Void)?

    @State private var isChanging = false
    @State private var showsTokens = false

    /// A summary of `totals` against `budgetCents` (0 is no budget). `name`
    /// titles it as a row; `subject` ("Dana", "The team") is who pauses at
    /// the limit. `onChangeBudget` offers Change… and `onResume` offers
    /// Resume while paused; either is absent when nil, as for a past month.
    public init(
        name: String? = nil, subject: String, totals: Hearth_V1_UsageTotals,
        budgetCents: Int64, isPaused: Bool,
        onChangeBudget: ((Int64) -> Void)?, onResume: (() -> Void)?
    ) {
        self.name = name
        self.subject = subject
        self.totals = totals
        self.budgetCents = budgetCents
        self.isPaused = isPaused
        self.onChangeBudget = onChangeBudget
        self.onResume = onResume
    }

    // MARK: Words

    /// An amount of US micro-dollars as the system writes dollars: "$18.40".
    public static func dollars(micros: Int64, locale: Locale = .current) -> String {
        (Decimal(micros) / 1_000_000).formatted(.currency(code: "USD").locale(locale))
    }

    /// A budget in cents: "$20" when whole, "$12.50" otherwise.
    public static func dollars(cents: Int64, locale: Locale = .current) -> String {
        let value = Decimal(cents) / 100
        let style = Decimal.FormatStyle.Currency.currency(code: "USD").locale(locale)
        return cents % 100 == 0
            ? value.formatted(style.precision(.fractionLength(0))) : value.formatted(style)
    }

    /// Every token counted, the plan's included: the unpriced headline.
    static func tokens(_ totals: Hearth_V1_UsageTotals) -> Int64 {
        totals.promptTokens + totals.completionTokens + totals.planTokens
    }

    static func count(_ number: Int64) -> String {
        number.formatted(.number.notation(.compactName))
    }

    var headline: String {
        totals.priced
            ? Self.dollars(micros: totals.costMicros) : "\(Self.count(Self.tokens(totals))) tokens"
    }

    var caption: String {
        let requests = "\(totals.requests) \(totals.requests == 1 ? "request" : "requests")"
        let steps = "\(totals.steps) \(totals.steps == 1 ? "step" : "steps")"
        return totals.priced
            ? "\(requests) · \(steps)" : "\(requests) · \(steps) · no price for these models"
    }

    /// The cost as a share of the budget, 0 when there is no budget or no
    /// price.
    var fraction: Double {
        guard budgetCents > 0, totals.priced else { return 0 }
        return Double(totals.costMicros) / Double(budgetCents * 10_000)
    }

    /// Neutral up to nine tenths of the budget, attention above, negative at
    /// it: the same rule as the handbook's meter.
    var tone: Tone {
        if fraction >= 1 { return .negative }
        return fraction > 0.9 ? .attention : .neutral
    }

    var budgetLine: String {
        let spent = Self.dollars(micros: totals.priced ? totals.costMicros : 0)
        return "\(spent) of \(Self.dollars(cents: budgetCents)) budget"
    }

    // MARK: View

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            if let name {
                HStack {
                    Text(name).font(Theme.headline)
                    Spacer(minLength: Theme.Spacing.sm)
                    pausedControls
                }
            }
            HStack(alignment: .firstTextBaseline) {
                Text(headline)
                    .font(Theme.title)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .monospacedDigit()
                Spacer(minLength: Theme.Spacing.sm)
                if name == nil { pausedControls }
            }
            Text(caption)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
                .monospacedDigit()
            tokensDisclosure
            budget
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder
    private var pausedControls: some View {
        if isPaused {
            HStack(spacing: Theme.Spacing.sm) {
                Pill("Paused", tone: .attention)
                if let onResume {
                    Button("Resume", action: onResume)
                        .controlSize(.small)
                }
            }
        }
    }

    private var tokensDisclosure: some View {
        DisclosureGroup("Tokens", isExpanded: $showsTokens) {
            VStack(spacing: Theme.Spacing.xs) {
                tokenRow("Prompt", totals.promptTokens)
                tokenRow("Completion", totals.completionTokens)
                tokenRow("Cached", totals.cachedTokens)
                if totals.planTokens > 0 { tokenRow("On your plan", totals.planTokens) }
            }
            .padding(.top, Theme.Spacing.xs)
        }
        .font(Theme.callout)
        .foregroundStyle(Theme.color(.textSecondary))
    }

    private func tokenRow(_ label: String, _ value: Int64) -> some View {
        HStack {
            Text(label)
            Spacer()
            Text(value.formatted()).monospacedDigit()
        }
        .font(Theme.caption)
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private var budget: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            if budgetCents > 0 {
                ProgressView(value: min(max(fraction, 0), 1))
                    .tint(tone == .neutral ? nil : Theme.color(tone))
            }
            HStack {
                Text(budgetCents > 0 ? budgetLine : "No budget")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(tone))
                    .monospacedDigit()
                Spacer(minLength: Theme.Spacing.sm)
                if onChangeBudget != nil {
                    Button(budgetCents > 0 ? "Change…" : "Set a Budget…") { isChanging = true }
                        .controlSize(.small)
                        .popover(isPresented: $isChanging, arrowEdge: .bottom) {
                            BudgetField(
                                subject: subject, cents: budgetCents,
                                onSave: { cents in
                                    isChanging = false
                                    onChangeBudget?(cents)
                                },
                                onCancel: { isChanging = false })
                        }
                }
            }
        }
        .accessibilityElement(children: .contain)
    }
}

extension UsageSummary {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case priced
        case unpriced
        case nearLimit
        case paused

        var totals: Hearth_V1_UsageTotals {
            var totals = Fixtures.usageSummary(month: "").bots[0].totals
            switch self {
            case .priced:
                totals.costMicros = 4_200_000
            case .unpriced:
                totals.priced = false
                totals.costMicros = 0
            case .nearLimit:
                break
            case .paused:
                totals.costMicros = 20_100_000
            }
            return totals
        }

        var budgetCents: Int64 { self == .unpriced ? 0 : 2000 }
    }

    /// A summary showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            subject: "Dana", totals: state.totals, budgetCents: state.budgetCents,
            isPaused: state == .paused, onChangeBudget: { _ in }, onResume: {})
    }
}

#Preview("Priced") { UsageSummary(previewState: .priced).frame(width: 300).padding() }
#Preview("Unpriced") { UsageSummary(previewState: .unpriced).frame(width: 300).padding() }
#Preview("Near the limit") { UsageSummary(previewState: .nearLimit).frame(width: 300).padding() }
#Preview("Paused") { UsageSummary(previewState: .paused).frame(width: 300).padding() }
