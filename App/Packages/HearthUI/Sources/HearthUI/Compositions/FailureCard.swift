// FailureCard is a notice that needs no decision: a run that failed, or, as
// the budget variant, a bot paused at its budget. The same header as every
// Inbox card, the core's plain-words reason in callout type with a symbol and
// the negative tone (the attention tone for a budget), then OK, which
// acknowledges it, and the prominent Retry (Resume for a budget) when the
// screen offers one. While the core answers the buttons are disabled with a
// small spinner; once it has, the outcome replaces them as a Pill. The tone
// always travels with the words and the symbol. It composes CardSurface,
// Avatar, Pill and Buttons; the card is one VoiceOver container named by the
// item's title.
// See docs/model.md#owner-interaction and docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// A failed run, or a budget reached, for the owner to note.
public struct FailureCard: View {
    let item: Hearth_V1_InboxItem
    let bot: Hearth_V1_Bot
    let state: InboxCardState
    let isDefault: Bool
    let onRetry: (() -> Void)?
    let onDismiss: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `item` about `bot`; `onRetry` is Retry (Resume for a
    /// budget), absent when nil, and `onDismiss` is OK.
    public init(
        item: Hearth_V1_InboxItem,
        bot: Hearth_V1_Bot,
        state: InboxCardState,
        isDefault: Bool = true,
        onRetry: (() -> Void)?,
        onDismiss: @escaping () -> Void
    ) {
        self.item = item
        self.bot = bot
        self.state = state
        self.isDefault = isDefault
        self.onRetry = onRetry
        self.onDismiss = onDismiss
    }

    var isBudget: Bool { item.kind == .budget }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            InboxCardHeader(bot: bot, date: item.createdAt.date)
            Label {
                Text(item.effect.isEmpty ? item.title : item.effect)
                    .fixedSize(horizontal: false, vertical: true)
            } icon: {
                Image(systemName: isBudget ? "gauge.with.needle" : "exclamationmark.triangle")
                    .accessibilityHidden(true)
            }
            .font(Theme.callout)
            .foregroundStyle(Theme.color(isBudget ? Tone.attention : Tone.negative))
            footer
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .cardSurface()
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: state)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(item.title)
    }

    @ViewBuilder
    private var footer: some View {
        switch state {
        case .open, .deciding:
            HStack(spacing: Theme.Spacing.sm) {
                Spacer(minLength: Theme.Spacing.sm)
                if state.isDeciding {
                    ProgressView()
                        .controlSize(.small)
                        .accessibilityLabel("Sending your answer")
                }
                Button("OK", action: onDismiss)
                    .accessibilityHint("Dismisses this notice")
                if let onRetry {
                    Button(isBudget ? "Resume" : "Retry", action: onRetry)
                        .buttonStyle(.borderedProminent)
                        .keyboardShortcut(isDefault ? KeyboardShortcut.defaultAction : nil)
                        .accessibilityHint(
                            isBudget ? "Lets \(bot.name) carry on" : "Runs the request again")
                }
            }
            .disabled(state.isDeciding)
        case .closed(let outcome, let tone):
            Pill(outcome, tone: tone)
                .transition(.blurReplace)
        }
    }
}

extension FailureCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case open
        case deciding
        case closed
        case budget

        var item: Hearth_V1_InboxItem {
            guard self == .budget else { return Fixtures.failure }
            var item = Fixtures.failure
            item.kind = .budget
            item.title = "Dana reached her budget"
            item.effect = "Dana has used this month's budget and is paused."
            return item
        }

        var state: InboxCardState {
            switch self {
            case .open, .budget: .open
            case .deciding: .deciding
            case .closed: .closed("Acknowledged", .neutral)
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            item: state.item, bot: Fixtures.bot, state: state.state, onRetry: {},
            onDismiss: {})
    }
}

#Preview("Open") { FailureCard(previewState: .open).frame(width: 600).padding() }
#Preview("Deciding") { FailureCard(previewState: .deciding).frame(width: 600).padding() }
#Preview("Closed") { FailureCard(previewState: .closed).frame(width: 600).padding() }
#Preview("Budget") { FailureCard(previewState: .budget).frame(width: 600).padding() }
