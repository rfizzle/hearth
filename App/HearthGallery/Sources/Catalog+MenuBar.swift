// The menu bar extra's panel, over PreviewClient and a MenuBarStore, at the
// system menu's width: Dana working and four items waiting; nobody working and
// nothing waiting; and a quick ask whose reply is streaming in. The status
// item's flame is the app target's and is not shown here.

import Foundation
import HearthDesign
import HearthFeatures
import HearthKit
import SwiftUI

extension Catalog {
    /// Dana's run started three minutes ago, so the elapsed time reads as
    /// the panel would show it.
    private static var recentRun: Hearth_V1_Run {
        var run = Fixtures.workingRun
        run.startedAt = .init(date: Date.now.addingTimeInterval(-180))
        return run
    }

    static func menuBarFilled() -> GalleryState {
        let store = menuBarStore(client: PreviewClient(latestRun: recentRun))
        Task { await store.load() }
        return menuBarState("both filled", store: store)
    }

    static func menuBarEmpty() -> GalleryState {
        let store = menuBarStore(client: noBotsClient)
        Task { await store.load() }
        return menuBarState("both empty", store: store)
    }

    /// The owner asked Dana; her reply has opened and two words have arrived,
    /// and the stream stays open.
    static func menuBarReplyStreaming() -> GalleryState {
        var reply = Fixtures.streamingReply
        reply.id = "message-12"
        reply.seq = 12
        reply.runID = Fixtures.sendResponse.run.id
        reply.content = ""
        var working = recentRun
        working.id = Fixtures.sendResponse.run.id
        let events: [Hearth_V1_SubscribeResponse.OneOf_Event] =
            [
                .runChanged(.with { $0.run = working }),
                .messageUpserted(.with { $0.message = reply }),
            ]
            + Fixtures.replyDeltas.prefix(2).map { text in
                .messageDelta(
                    .with {
                        $0.conversationID = reply.conversationID
                        $0.messageID = reply.id
                        $0.text = text
                    })
            }
        let scripted = events.enumerated().map { index, event in
            Hearth_V1_SubscribeResponse.with {
                $0.sequence = UInt64(index + 1)
                $0.event = event
            }
        }
        let relay = EventRelay(subscribe: { after in
            AsyncThrowingStream { continuation in
                for event in scripted where event.sequence > after { continuation.yield(event) }
                // Left open on purpose: the reply is still streaming.
                Task {
                    try? await Task.sleep(for: .seconds(86_400))
                    continuation.finish()
                }
            }
        })
        let store = menuBarStore(client: PreviewClient(), relay: relay)
        Task {
            await store.load()
            store.pick(Fixtures.bot)
            store.draft = "Add a section on running costs."
            await store.ask()
            relay.start()
        }
        return menuBarState("reply streaming", store: store)
    }

    private static func menuBarStore(
        client: any HearthClient, relay: EventRelay? = nil
    ) -> MenuBarStore {
        let relay = relay ?? EventRelay(client: client)
        let inbox = InboxStore(client: client, relay: relay)
        let bots = BotListStore(client: client, relay: relay)
        var conversations: [String: ConversationStore] = [:]
        return MenuBarStore(bots: bots, inbox: inbox) { bot in
            if let made = conversations[bot.dmConversationID] { return made }
            let made = ConversationStore(
                client: client, relay: relay, conversationID: bot.dmConversationID, inbox: inbox)
            conversations[bot.dmConversationID] = made
            return made
        }
    }

    private static func menuBarState(_ name: String, store: MenuBarStore) -> GalleryState {
        GalleryState(
            name: name, width: Theme.Size.menuExtraWidth,
            view: AnyView(
                MenuBarPanel(
                    store: store, orgName: Fixtures.org.name,
                    onOpenConversation: { _ in }, onSettings: {})))
    }
}
