// BotList is the bots' part of the sidebar: the org-level bots, the Chief of
// Staff first, under an "Office" header, then the rest under "Your team", one
// row each with an avatar, the name, the role and, while a bot is working,
// paused or waiting for the owner, a status dot. It is sections, not a list:
// SidebarList holds them in its List with the selection. Rows are List rows,
// so selection, keyboard and VoiceOver are the system's, and a row reads as one
// element, saying "waiting for you" as its value while the bot waits. The role
// uses the system's .secondary style rather than the text token so sidebar
// vibrancy applies over the glass pane. With nobody on the team its section
// shows only the header; the invitation to hire is TeamEmptyView, which the
// window shows in the detail column. A bot's context menu offers Access… when
// the window gives it.
// See docs/design.md#window-anatomy and docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The bots' sections of the sidebar.
public struct BotList: View {
    let store: BotListStore
    let onAccess: ((Hearth_V1_Bot) -> Void)?

    /// The Office and the team over `store`. A bot's context menu offers
    /// Access… when `onAccess` is given.
    public init(store: BotListStore, onAccess: ((Hearth_V1_Bot) -> Void)? = nil) {
        self.store = store
        self.onAccess = onAccess
    }

    public var body: some View {
        if !store.office.isEmpty {
            Section("Office") {
                ForEach(store.office, id: \.id) { row($0) }
            }
        }
        Section("Your team") {
            ForEach(store.team, id: \.id) { row($0) }
        }
    }

    private func row(_ bot: Hearth_V1_Bot) -> some View {
        BotRow(bot: bot, activity: store.activity(of: bot))
            .tag(SidebarSelection.bot(bot.id))
            .contextMenu {
                if let onAccess {
                    Button("Access…", systemImage: "lock.open") { onAccess(bot) }
                }
            }
    }
}

/// The detail column's empty state when nobody is hired yet.
public struct TeamEmptyView: View {
    let onHire: () -> Void

    public init(onHire: @escaping () -> Void) {
        self.onHire = onHire
    }

    public var body: some View {
        ContentUnavailableView {
            Label("Nobody here yet", systemImage: "person.2")
        } description: {
            Text("Hire someone to get started.")
        } actions: {
            Button("Hire Someone…", action: onHire)
                .buttonStyle(.borderedProminent)
        }
    }
}

private struct BotRow: View {
    let bot: Hearth_V1_Bot
    let activity: BotListStore.Activity?

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Avatar(bot: bot)
            VStack(alignment: .leading, spacing: 0) {
                Text(bot.name).font(Theme.body)
                Text(bot.role).font(Theme.caption).foregroundStyle(.secondary)
            }
            Spacer(minLength: Theme.Spacing.sm)
            switch activity {
            case .working:
                StatusDot(.positive, label: "Working", pulses: true)
            case .waiting:
                StatusDot(.attention, label: "Waiting")
            case .paused:
                Image(systemName: "pause.circle")
                    .foregroundStyle(Theme.color(Tone.attention))
                    .accessibilityLabel("Paused")
            case nil:
                EmptyView()
            }
        }
        .accessibilityElement(children: .combine)
        .accessibilityValue(activity?.spoken ?? "")
    }
}
