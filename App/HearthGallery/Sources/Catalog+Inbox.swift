// What waits on the owner: the decision bar in its wide and narrow layouts,
// the approval and failure cards in each state (the question's card is with
// the widgets), the Inbox with
// items, empty and as the sidebar's row, and a conversation whose run waits
// on the owner with the card in the thread.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var inbox: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "DecisionBar",
                states: DecisionBar.PreviewState.allCases.map { state in
                    GalleryState(
                        name: state.rawValue, width: state.width,
                        view: AnyView(DecisionBar(previewState: state)))
                }),
            GalleryPage(
                package: package, name: "ApprovalCard",
                states: GalleryState.each(ApprovalCard.PreviewState.self, width: 600) {
                    ApprovalCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "FailureCard",
                states: GalleryState.each(FailureCard.PreviewState.self, width: 600) {
                    FailureCard(previewState: $0)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "InboxScreen",
                states: [inboxWithItems(), inboxEmpty(), sidebarWithInbox()]),
        ]
    }

    /// PreviewClient's four open items, one a question with a control.
    static func inboxWithItems() -> GalleryState {
        let client = PreviewClient()
        let store = InboxStore(client: client, relay: EventRelay(client: client))
        return GalleryState(
            name: "items", width: 680,
            view: AnyView(
                InboxScreen(store: store, bots: [Fixtures.bot]).frame(height: 1180)))
    }

    /// Every item closes after the screen's own fetch, as it would live.
    static func inboxEmpty() -> GalleryState {
        let client = PreviewClient()
        let closes = Fixtures.inbox.map { item in
            var closed = item
            closed.state = .closed
            return Hearth_V1_SubscribeResponse.with {
                $0.event = .inboxChanged(.with { $0.item = closed })
            }
        }
        let relay = EventRelay(subscribe: { _ in
            AsyncThrowingStream { continuation in
                let task = Task {
                    try? await Task.sleep(for: .milliseconds(300))
                    closes.forEach { continuation.yield($0) }
                }
                continuation.onTermination = { _ in task.cancel() }
            }
        })
        relay.start()
        let store = InboxStore(client: client, relay: relay)
        return GalleryState(
            name: "empty", width: 680,
            view: AnyView(InboxScreen(store: store, bots: [Fixtures.bot]).frame(height: 320)))
    }

    /// The sidebar with the Inbox row above the team, three items open and
    /// the Inbox selected.
    static func sidebarWithInbox() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let bots = BotListStore(client: client, relay: relay)
        bots.selected = .inbox
        let inbox = InboxStore(client: client, relay: relay)
        return GalleryState(
            name: "sidebar", width: Theme.Size.sidebarIdeal,
            view: AnyView(
                BotList(store: bots, inbox: inbox, onHire: {}).frame(height: 220)))
    }
}

extension Catalog {
    /// Dana has parked at the delete: the run waits, the request is in the
    /// thread and the Inbox; the stream stays open.
    static func conversationWaiting() -> GalleryState {
        let client = PreviewClient()
        let arrived = Array(Fixtures.parkEvents.prefix(3))
        let relay = EventRelay(subscribe: { after in
            AsyncThrowingStream { continuation in
                for event in arrived where event.sequence > after { continuation.yield(event) }
                // Left open on purpose: the run is still waiting.
                Task {
                    try? await Task.sleep(for: .seconds(86_400))
                    continuation.finish()
                }
            }
        })
        let inbox = InboxStore(client: client, relay: relay)
        let store = ConversationStore(
            client: client, relay: relay, conversationID: Fixtures.dmConversation.id,
            inbox: inbox)
        Task {
            await store.load()
            relay.start()
        }
        return GalleryState(
            name: "waiting", width: 600,
            view: AnyView(
                ConversationScreen(store: store, bot: Fixtures.bot).frame(height: 520)))
    }
}
