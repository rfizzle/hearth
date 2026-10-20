// InboxItemCard picks the card for an Inbox item and wires its buttons to the
// store: an ApprovalCard with the four decisions, a PromptCard with the
// question's control and the reply (and Change while the core still takes a
// new answer), a FailureCard with Retry and OK (a pause has Resume and OK, a
// warning OK alone). The Inbox screen and the conversation draw the same
// card through it, so an item looks and acts alike in both places.
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

    /// The card's prominent button: Retry for a failure, Resume for a pause.
    private var onRetry: (() -> Void)? {
        if item.kind == .failure { return { Task { await store.retry(item) } } }
        if store.canResume(item) { return { Task { await store.resume(item) } } }
        return nil
    }

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
                item: store.answeredItem(item) ?? item, bot: bot, state: state,
                isDefault: isDefault, canChange: store.canChange(item),
                onAnswer: { text, value in
                    Task { await store.answer(itemID: item.id, text: text, value: value) }
                })
        default:
            FailureCard(
                item: item, bot: bot, state: state, isDefault: isDefault,
                onRetry: onRetry,
                onDismiss: { Task { await store.acknowledge(item) } })
        }
    }
}
