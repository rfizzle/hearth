// What waits on the owner: the decision bar in its wide and narrow layouts,
// the approval, question and failure cards in each state, and a conversation
// whose run waits on the owner with the card in the thread.

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
                package: package, name: "QuestionCard",
                states: GalleryState.each(QuestionCard.PreviewState.self, width: 600) {
                    QuestionCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "FailureCard",
                states: GalleryState.each(FailureCard.PreviewState.self, width: 600) {
                    FailureCard(previewState: $0)
                }),
        ]
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
