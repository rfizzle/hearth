// BotList is the sidebar's list of bots under a "Your team" header, one row
// each with an avatar, the name, the role and, while a bot is working or
// paused, a status dot; the store holds the selection. Given the Inbox, a row
// above the team opens it, carrying how many items are open in a Pill in the
// attention tone, and nothing when none are; its label says the count too. Rows are List rows, so
// selection, keyboard and VoiceOver are the system's, and a row reads as one
// element. The role uses the system's .secondary style rather than the text
// token so sidebar vibrancy applies over the glass pane. With nobody hired the
// sidebar shows only the header; the invitation to hire is TeamEmptyView, which
// the window shows in the detail column. A new bot slides into the list.
// See docs/design.md#window-anatomy, docs/design.md#motion and
// docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The bots in the sidebar.
public struct BotList: View {
    let store: BotListStore
    let inbox: InboxStore?
    let onHire: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// The sidebar over `store`, with the Inbox's row when `inbox` is given.
    public init(store: BotListStore, inbox: InboxStore? = nil, onHire: @escaping () -> Void) {
        self.store = store
        self.inbox = inbox
        self.onHire = onHire
    }

    public var body: some View {
        @Bindable var store = store
        List(selection: $store.selected) {
            if let inbox {
                Section {
                    InboxRow(openCount: inbox.openCount)
                        .tag(BotListStore.Selection.inbox)
                }
            }
            Section("Your team") {
                ForEach(store.bots, id: \.id) { bot in
                    BotRow(bot: bot)
                        .tag(BotListStore.Selection.bot(bot.id))
                }
            }
        }
        .listStyle(.sidebar)
        .animation(
            Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.bots.map(\.id)
        )
        .task { await store.load() }
        .task { await inbox?.load() }
    }
}

/// The sidebar's row for the Inbox, with the open count.
struct InboxRow: View {
    let openCount: Int

    var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Label("Inbox", systemImage: "tray")
                .font(Theme.body)
            Spacer(minLength: Theme.Spacing.sm)
            if let count = Self.count(openCount) {
                Pill(count, tone: .attention)
            }
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("Inbox")
        .accessibilityValue(Self.spoken(openCount))
    }

    /// The count the row shows, or nil when nothing is open.
    static func count(_ openCount: Int) -> String? {
        openCount > 0 ? String(openCount) : nil
    }

    /// The count as VoiceOver says it.
    static func spoken(_ openCount: Int) -> String {
        switch openCount {
        case 0: "Nothing waiting"
        case 1: "1 item waiting"
        default: "\(openCount) items waiting"
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
