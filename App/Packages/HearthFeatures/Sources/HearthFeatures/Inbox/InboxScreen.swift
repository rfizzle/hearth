// InboxScreen is everything that waits on the owner, as cards in a readable
// column: "Needs a decision" (approvals and questions, a run waits on each)
// over "To note" (failures and budgets), newest first, open items only. Each
// card is the one the thread draws for the same item. The newest card that
// needs a decision (else the newest card) has its main button answer to
// Return, and no other, so one key never means two things. A card the owner
// answered shows its outcome until the core closes the item, then leaves the
// column; nothing else moves. With nothing open it is the system's empty
// state.
// See docs/model.md#owner-interaction, docs/design.md#empty-states-and-first-run
// and docs/design.md#motion

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// What waits on the owner.
public struct InboxScreen: View {
    let store: InboxStore
    let bots: [Hearth_V1_Bot]

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// The Inbox over `store`; `bots` name and draw who asked.
    public init(store: InboxStore, bots: [Hearth_V1_Bot]) {
        self.store = store
        self.bots = bots
    }

    public var body: some View {
        content
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: store.items.isEmpty
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .navigationTitle("Inbox")
            .task { await store.load() }
    }

    @ViewBuilder
    private var content: some View {
        if !store.loaded, let error = store.error {
            ContentUnavailableView {
                Label("Couldn't load the Inbox", systemImage: "exclamationmark.triangle")
            } description: {
                Text(error)
            } actions: {
                Button("Try Again") { Task { await store.load() } }
            }
        } else if !store.loaded {
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if store.items.isEmpty {
            ContentUnavailableView(
                "Nothing waiting", systemImage: "tray",
                description: Text("When someone needs you, it shows up here.")
            )
            .transition(.opacity)
        } else {
            list
        }
    }

    private var list: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                ForEach(store.sections) { section in
                    Text(section.rawValue)
                        .font(Theme.headline)
                        .foregroundStyle(Theme.color(.textSecondary))
                        .accessibilityAddTraits(.isHeader)
                    ForEach(store.items(in: section), id: \.id) { item in
                        InboxItemCard(
                            store: store, item: item, bot: bot(item.botID),
                            isDefault: item.id == defaultID
                        )
                        .transition(.asymmetric(insertion: .identity, removal: .opacity))
                    }
                }
            }
            .animation(
                Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.items.map(\.id)
            )
            .frame(maxWidth: Theme.Size.readableWidth)
            .padding(Theme.Spacing.lg)
            .frame(maxWidth: .infinity)
        }
    }

    /// The card whose main button answers to Return.
    private var defaultID: String? {
        (store.items(in: .needsDecision).first ?? store.items.first)?.id
    }

    /// The bot `id` names, or a stand-in while the team has not loaded.
    private func bot(_ id: String) -> Hearth_V1_Bot {
        bots.first { $0.id == id } ?? .with { $0.name = "Someone" }
    }
}
