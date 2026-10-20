// MenuBarPanel is the menu bar extra's panel over a MenuBarStore: the
// MenuBarPopover with its quick ask, and QuickAskPanel is the quick ask alone,
// for the window the global shortcut opens. Both remember who the owner last
// asked in the app's defaults, so the extra and the shortcut agree. Opening
// the window and Settings are the app's, handed in.
// See docs/design.md#window-anatomy

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The extra's panel.
public struct MenuBarPanel: View {
    let store: MenuBarStore
    let orgName: String
    let onOpenConversation: (String?) -> Void
    let onSettings: () -> Void

    @AppStorage(QuickAskPanel.botKey) private var rememberedBotID = ""

    /// A panel over `store`; an item about no bot is named `orgName`.
    /// `onOpenConversation` opens the window at a conversation, or as it was
    /// when given nil, and `onSettings` opens Settings.
    public init(
        store: MenuBarStore, orgName: String,
        onOpenConversation: @escaping (String?) -> Void,
        onSettings: @escaping () -> Void
    ) {
        self.store = store
        self.orgName = orgName
        self.onOpenConversation = onOpenConversation
        self.onSettings = onSettings
    }

    public var body: some View {
        MenuBarPopover(
            ask: QuickAskField(
                store: store, remembered: $rememberedBotID, open: onOpenConversation),
            working: store.working, waiting: store.waiting, orgName: orgName,
            onOpenConversation: { onOpenConversation($0) },
            onOpenHearth: { onOpenConversation(nil) },
            onSettings: onSettings
        )
        .onAppear { if !rememberedBotID.isEmpty { store.chosenBotID = rememberedBotID } }
    }
}

/// The quick ask alone: the field and its reply, at the extra's width.
public struct QuickAskPanel: View {
    /// Where the bot last asked is remembered.
    static let botKey = "quickAskBotID"

    let store: MenuBarStore
    let onOpenConversation: (String?) -> Void

    @AppStorage(Self.botKey) private var rememberedBotID = ""

    /// The quick ask over `store`; `onOpenConversation` opens the window at
    /// the reply's conversation.
    public init(store: MenuBarStore, onOpenConversation: @escaping (String?) -> Void) {
        self.store = store
        self.onOpenConversation = onOpenConversation
    }

    public var body: some View {
        QuickAskField(store: store, remembered: $rememberedBotID, open: onOpenConversation)
            .padding(Theme.Spacing.md)
            .frame(width: Theme.Size.menuExtraWidth, alignment: .leading)
            .onAppear { if !rememberedBotID.isEmpty { store.chosenBotID = rememberedBotID } }
    }
}

extension QuickAskField {
    /// The field over `store`, remembering the bot picked in `remembered`.
    @MainActor
    fileprivate init(
        store: MenuBarStore, remembered: Binding<String>, open: @escaping (String?) -> Void
    ) {
        @Bindable var store = store
        self.init(
            bots: store.bots.bots, bot: store.bot, draft: $store.draft,
            reply: store.reply, replyAuthor: store.replyBot, canSend: store.canAsk,
            error: store.error,
            onPick: { bot in
                store.pick(bot)
                remembered.wrappedValue = bot.id
            },
            onSend: { Task { await store.ask() } },
            onOpen: { open(store.replyConversationID) })
    }
}
