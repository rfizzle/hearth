// MenuBarPopover is the menu bar extra's panel: a glance, not a dashboard. One
// column at a system menu's width with a menu's manners: no card and no glass
// (the panel is the system's material), primary and secondary text so it takes
// the panel's vibrancy, and nothing scrolling at rest. From the top: the
// QuickAskField; "Working", a row per bot whose run works (Avatar, name, the
// run's status line under it, the time it has run); "Needs you · N", a row per
// open Inbox item (Avatar, name, the item's title, a symbol for its kind);
// every row is a button with the system's hover that opens the window at its
// conversation; then a footer of Open Hearth, Pause all (disabled until
// hearth can pause everyone, with its reason as help) and Settings…. Rows
// arrive from the core, so they appear without motion. Each row reads as one
// VoiceOver element, headers are headers, and avatars are hidden since the
// name says who.
// See docs/design.md#window-anatomy and docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import SwiftUI

/// The menu bar extra's panel.
public struct MenuBarPopover: View {
    /// A bot with a run working.
    public struct Working: Identifiable, Equatable, Sendable {
        public let bot: Hearth_V1_Bot
        public let run: Hearth_V1_Run
        public var id: String { bot.id }

        /// `bot`, working on `run`.
        public init(bot: Hearth_V1_Bot, run: Hearth_V1_Run) {
            self.bot = bot
            self.run = run
        }
    }

    /// An open Inbox item and the bot it is about, nil for the whole org.
    public struct Waiting: Identifiable, Equatable, Sendable {
        public let item: Hearth_V1_InboxItem
        public let bot: Hearth_V1_Bot?
        public var id: String { item.id }

        /// `item`, about `bot`.
        public init(item: Hearth_V1_InboxItem, bot: Hearth_V1_Bot?) {
            self.item = item
            self.bot = bot
        }
    }

    let ask: QuickAskField
    let working: [Working]
    let waiting: [Waiting]
    let orgName: String
    let onOpenConversation: (String) -> Void
    let onOpenHearth: () -> Void
    let onSettings: () -> Void

    /// A panel with `ask` at the top, then `working` and `waiting`; an item
    /// about no bot is named `orgName`. `onOpenConversation` opens the window
    /// at a row's conversation (an unknown one opens the Inbox),
    /// `onOpenHearth` opens the window, and `onSettings` opens Settings.
    public init(
        ask: QuickAskField,
        working: [Working],
        waiting: [Waiting],
        orgName: String,
        onOpenConversation: @escaping (String) -> Void,
        onOpenHearth: @escaping () -> Void,
        onSettings: @escaping () -> Void
    ) {
        self.ask = ask
        self.working = working
        self.waiting = waiting
        self.orgName = orgName
        self.onOpenConversation = onOpenConversation
        self.onOpenHearth = onOpenHearth
        self.onSettings = onSettings
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            ask
            Divider()
            section("Working") {
                if working.isEmpty {
                    empty("Everyone's free")
                } else {
                    ForEach(working) { entry in
                        WorkingRow(working: entry) { onOpenConversation(entry.run.conversationID) }
                    }
                }
            }
            Divider()
            section(waiting.isEmpty ? "Needs you" : "Needs you · \(waiting.count)") {
                if waiting.isEmpty {
                    empty("Nothing waiting")
                } else {
                    ForEach(waiting) { entry in
                        WaitingRow(waiting: entry, orgName: orgName) {
                            onOpenConversation(entry.item.conversationID)
                        }
                    }
                }
            }
            Divider()
            footer
        }
        .padding(Theme.Spacing.md)
        .frame(width: Theme.Size.menuExtraWidth, alignment: .leading)
    }

    private func section<Content: View>(
        _ title: String, @ViewBuilder content: () -> Content
    ) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            Text(title)
                .font(Theme.caption.weight(.semibold))
                .foregroundStyle(.secondary)
                .accessibilityAddTraits(.isHeader)
            content()
        }
    }

    private func empty(_ text: String) -> some View {
        Text(text)
            .font(Theme.callout)
            .foregroundStyle(.secondary)
    }

    private var footer: some View {
        HStack(spacing: Theme.Spacing.xs) {
            Button("Open Hearth", action: onOpenHearth)
            Button("Pause all") {}
                .disabled(true)
                .help("Hearth can't pause everyone at once yet.")
            Spacer(minLength: Theme.Spacing.xs)
            Button("Settings…", action: onSettings)
        }
        .buttonStyle(.accessoryBar)
        .font(Theme.caption)
    }

    /// The symbol beside an item: what kind of wait it is.
    static func symbol(for kind: Hearth_V1_InboxKind) -> String {
        switch kind {
        case .approval: "hand.raised"
        case .question: "questionmark.circle"
        case .budget: "pause.circle"
        default: "exclamationmark.triangle"
        }
    }
}

/// A bot at work: who, the run's line, and how long it has run, as a button
/// that opens the conversation.
private struct WorkingRow: View {
    let working: MenuBarPopover.Working
    let open: () -> Void

    var body: some View {
        Button(action: open) {
            row
                .frame(maxWidth: .infinity, alignment: .leading)
                .contentShape(Rectangle())
        }
        .buttonStyle(.accessoryBar)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(working.bot.name), working, \(elapsed)")
        .accessibilityHint("Opens the conversation")
        .accessibilityAddTraits(.isButton)
    }

    /// How long the run has worked, as VoiceOver says it: "3 minutes".
    private var elapsed: String {
        let seconds = max(0, Date.now.timeIntervalSince(working.run.startedAt.date))
        return Duration.seconds(seconds).formatted(
            .units(allowed: [.hours, .minutes, .seconds], width: .wide, maximumUnitCount: 1))
    }

    private var row: some View {
        HStack(alignment: .top, spacing: Theme.Spacing.sm) {
            Avatar(bot: working.bot).accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(working.bot.name)
                    .font(Theme.body)
                    .foregroundStyle(.primary)
                RunStatusLine(run: working.run, botName: working.bot.name)
            }
            Spacer(minLength: Theme.Spacing.sm)
            Text(working.run.startedAt.date, style: .relative)
                .font(Theme.caption)
                .foregroundStyle(.secondary)
                .monospacedDigit()
        }
    }
}

/// An item waiting on the owner, as a button that opens it.
private struct WaitingRow: View {
    let waiting: MenuBarPopover.Waiting
    let orgName: String
    let open: () -> Void

    var body: some View {
        Button(action: open) {
            HStack(alignment: .top, spacing: Theme.Spacing.sm) {
                avatar.accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 0) {
                    Text(waiting.bot?.name ?? orgName)
                        .font(Theme.body)
                        .foregroundStyle(.primary)
                    Text(waiting.item.title)
                        .font(Theme.callout)
                        .foregroundStyle(.secondary)
                        .lineLimit(2)
                        .multilineTextAlignment(.leading)
                }
                Spacer(minLength: Theme.Spacing.sm)
                Image(systemName: MenuBarPopover.symbol(for: waiting.item.kind))
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .contentShape(Rectangle())
        }
        .buttonStyle(.accessoryBar)
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(waiting.item.title)
        .accessibilityHint("Opens the conversation")
        .accessibilityAddTraits(.isButton)
    }

    @ViewBuilder
    private var avatar: some View {
        if let bot = waiting.bot {
            Avatar(bot: bot)
        } else {
            Avatar(.symbol("flame"))
        }
    }
}

extension MenuBarPopover {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case bothFilled = "both filled"
        case bothEmpty = "both empty"
        case replyStreaming = "reply streaming"

        var working: [Working] {
            guard self != .bothEmpty else { return [] }
            var run = Fixtures.workingRun
            run.startedAt = .init(date: Date.now.addingTimeInterval(-180))
            return [Working(bot: Fixtures.bot, run: run)]
        }

        var waiting: [Waiting] {
            guard self != .bothEmpty else { return [] }
            return [Fixtures.approval, Fixtures.failure].map {
                Waiting(item: $0, bot: Fixtures.bot)
            }
        }
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            ask: QuickAskField(
                previewState: state == .replyStreaming ? .replyStreaming : .empty),
            working: state.working, waiting: state.waiting, orgName: Fixtures.org.name,
            onOpenConversation: { _ in }, onOpenHearth: {}, onSettings: {})
    }
}

#Preview("Both filled") { MenuBarPopover(previewState: .bothFilled) }
#Preview("Both empty") { MenuBarPopover(previewState: .bothEmpty) }
#Preview("Reply streaming") { MenuBarPopover(previewState: .replyStreaming) }
