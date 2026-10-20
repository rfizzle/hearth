// BotList is the sidebar's list of bots under a "Your team" header, one row
// each with an avatar, the name, the role and, while a bot is working or
// paused, a status dot; the store holds the selection. Rows are List rows, so
// selection, keyboard and VoiceOver are the system's, and a row reads as one
// element. The role uses the system's .secondary style rather than the text
// token so sidebar vibrancy applies over the glass pane. With nobody hired the
// sidebar shows only the header; the invitation to hire is TeamEmptyView, which
// the window shows in the detail column.
// See docs/design.md#window-anatomy

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The bots in the sidebar.
public struct BotList: View {
    let store: BotListStore
    let onHire: () -> Void

    public init(store: BotListStore, onHire: @escaping () -> Void) {
        self.store = store
        self.onHire = onHire
    }

    public var body: some View {
        @Bindable var store = store
        List(selection: $store.selection) {
            Section("Your team") {
                ForEach(store.bots, id: \.id) { bot in
                    BotRow(bot: bot)
                }
            }
        }
        .listStyle(.sidebar)
        .task { await store.load() }
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

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Avatar(bot: bot)
            VStack(alignment: .leading, spacing: 0) {
                Text(bot.name).font(Theme.body)
                Text(bot.role).font(Theme.caption).foregroundStyle(.secondary)
            }
            Spacer(minLength: Theme.Spacing.sm)
            switch bot.status {
            case .working:
                StatusDot(.positive, label: "Working", pulses: true)
            case .paused:
                StatusDot(.attention, label: "Paused")
            default:
                EmptyView()
            }
        }
        .accessibilityElement(children: .combine)
    }
}
