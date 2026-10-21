// ApprovalCard is one tool call a bot may not make without the owner: a card
// with the bot's avatar, name and the time it asked, the exact effect in the
// tool's own words (never the bot's paraphrase), a collapsed disclosure to the
// call's arguments, and the DecisionBar. While the core answers the bar is
// disabled with a small spinner; once it has, the bar's place holds the
// outcome as a Pill, which replaces it in place. It composes CardSurface,
// Avatar, DisclosureRow, DecisionBar and Pill; the card is one VoiceOver
// container named by the item's title, the header reads as one element, and
// the bar's buttons are its own. The same card stands in the Inbox and in the
// thread where the run parked.
// See docs/model.md#8-approvals and docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import SwiftUI

/// Where an Inbox card is: waiting for the owner, waiting for the core, or
/// answered with the outcome the core gave in plain words.
public enum InboxCardState: Equatable, Sendable {
    case open
    case deciding
    case closed(String, Tone)

    var isDeciding: Bool { self == .deciding }
}

/// An approval for one tool call.
public struct ApprovalCard: View {
    let item: Hearth_V1_InboxItem
    let bot: Hearth_V1_Bot
    let state: InboxCardState
    let canAlwaysAllow: Bool
    let isDefault: Bool
    let onDecide: (Hearth_V1_Decision, String) -> Void

    @State private var showsArguments = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `item`, which `bot` asked for; `canAlwaysAllow` offers the
    /// standing grant and `isDefault` binds Allow Once to Return.
    public init(
        item: Hearth_V1_InboxItem,
        bot: Hearth_V1_Bot,
        state: InboxCardState,
        canAlwaysAllow: Bool,
        isDefault: Bool = true,
        onDecide: @escaping (Hearth_V1_Decision, String) -> Void
    ) {
        self.item = item
        self.bot = bot
        self.state = state
        self.canAlwaysAllow = canAlwaysAllow
        self.isDefault = isDefault
        self.onDecide = onDecide
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            InboxCardHeader(bot: bot, date: item.createdAt.date)
            Text(Self.effect(item))
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
            DisclosureRow(isExpanded: $showsArguments) {
                Label("Details", systemImage: ToolCallRow.symbol(for: item.call.name))
            } content: {
                Text(item.call.argumentsJson)
                    .font(Theme.monospaced)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(Theme.Spacing.sm)
                    .background(
                        RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                            .fill(Theme.color(.surfaceSecondary))
                    )
                    .accessibilityLabel("Arguments")
                    .accessibilityValue(item.call.argumentsJson)
            }
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
            DecisionBar(
                botName: bot.name, canAlwaysAllow: canAlwaysAllow,
                isDeciding: state.isDeciding, isDefault: isDefault, onDecide: onDecide)
        case .closed(let outcome, let tone):
            Pill(outcome, tone: tone)
                .transition(.blurReplace)
        }
    }

    /// What the call will do: the core's effect string, else the call's own
    /// summary.
    static func effect(_ item: Hearth_V1_InboxItem) -> String {
        item.effect.isEmpty ? ToolCallRow.summary(item.call) : item.effect
    }
}

/// The head of every Inbox card: who asked and when.
struct InboxCardHeader: View {
    let bot: Hearth_V1_Bot
    let date: Date

    var body: some View {
        HStack(alignment: .center, spacing: Theme.Spacing.sm) {
            Avatar(bot: bot)
                .accessibilityHidden(true)
            Text(bot.name)
                .font(Theme.headline)
                .foregroundStyle(Theme.color(.textPrimary))
            Spacer(minLength: Theme.Spacing.sm)
            Text(date.formatted(.dateTime.hour().minute()))
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
        }
        .accessibilityElement(children: .combine)
    }
}

extension ApprovalCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case open
        case alwaysAllowOffered
        case deciding
        case closed

        /// The fixture's delete, or a write the owner may allow for good.
        var item: Hearth_V1_InboxItem {
            guard self == .alwaysAllowOffered else { return Fixtures.approval }
            var item = Fixtures.approval
            item.title = "Dana wants to write a file"
            item.effect =
                "Writes Competitor pricing brief.md in Dana's folder, replacing what is there."
            item.call.name = "files.write"
            item.call.summary = "Write Competitor pricing brief.md in Dana's folder"
            return item
        }

        var state: InboxCardState {
            switch self {
            case .open, .alwaysAllowOffered: .open
            case .deciding: .deciding
            case .closed: .closed("Allowed once", .positive)
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            item: state.item, bot: Fixtures.bot, state: state.state,
            canAlwaysAllow: state == .alwaysAllowOffered, onDecide: { _, _ in })
    }
}

#Preview("Open") { ApprovalCard(previewState: .open).frame(width: 600).padding() }
#Preview("Always Allow offered") {
    ApprovalCard(previewState: .alwaysAllowOffered).frame(width: 600).padding()
}
#Preview("Deciding") { ApprovalCard(previewState: .deciding).frame(width: 600).padding() }
#Preview("Closed") { ApprovalCard(previewState: .closed).frame(width: 600).padding() }
