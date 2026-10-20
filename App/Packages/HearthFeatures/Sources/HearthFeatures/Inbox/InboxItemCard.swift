// InboxItemCard picks the card for an Inbox item and wires its buttons to the
// store: an ApprovalCard with the four decisions, a PromptCard with the
// question's control and the reply, a FailureCard with Retry and OK (OK
// alone for a budget until resuming is a call). The Inbox screen and the
// conversation draw the same card through it, so an item looks and acts
// alike in both places.
// See docs/model.md#owner-interaction

import HearthKit
import HearthUI
import SwiftUI

/// The card for one open item, acting through the store.
struct InboxItemCard: View {
    let store: InboxStore
    let item: Hearth_V1_InboxItem
    let bot: Hearth_V1_Bot
    /// Whether the card's main button answers to Return.
    let isDefault: Bool

    var body: some View {
        let state = store.state(of: item)
        switch item.kind {
        case .approval:
            ApprovalCard(
                item: item, bot: bot, state: state, canAlwaysAllow: store.canAlwaysAllow(item),
                isDefault: isDefault,
                onDecide: { decision, reason in
                    Task { await store.decide(item, decision, reason: reason) }
                })
        case .question:
            PromptCard(
                item: item, bot: bot, state: state, isDefault: isDefault,
                onAnswer: { text, _ in Task { await store.answer(item, text) } })
        default:
            FailureCard(
                item: item, bot: bot, state: state, isDefault: isDefault,
                onRetry: item.kind == .failure ? { Task { await store.retry(item) } } : nil,
                onDismiss: { Task { await store.acknowledge(item) } })
        }
    }
}
