// BotList is the sidebar's list of bots, one row each with an avatar, the name
// and the role, and the store's selection. With nobody hired it offers the
// hire button instead. An avatar that is plain ASCII is an SF Symbol name, any
// other text is an emoji, and an empty one shows the name's initials. Rows are
// List rows, so selection, keyboard and VoiceOver are the system's.

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
        Group {
            if store.showsHireAffordance {
                VStack(spacing: 12) {
                    Text("Nobody here yet").font(.headline)
                    Button("Hire someone", action: onHire)
                        .buttonStyle(.borderedProminent)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                List(store.bots, id: \.id, selection: $store.selection) { bot in
                    BotRow(bot: bot)
                }
            }
        }
        .task { await store.load() }
        .task { await store.run() }
    }
}

private struct BotRow: View {
    let bot: Hearth_V1_Bot

    var body: some View {
        HStack {
            avatar.frame(width: 28)
            VStack(alignment: .leading) {
                Text(bot.name).font(.headline)
                Text(bot.role).foregroundStyle(.secondary)
            }
        }
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private var avatar: some View {
        if bot.avatar.isEmpty {
            Text(String(bot.name.prefix(1)).uppercased())
        } else if bot.avatar.allSatisfy(\.isASCII) {
            Image(systemName: bot.avatar)
        } else {
            Text(bot.avatar)
        }
    }
}
