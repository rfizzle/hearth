// RequestRow is an approval request or a question where the run parked in the
// thread. While its Inbox item is open it is the item's card, acting through
// the Inbox, so it looks and acts as it does there; once a question has been
// answered and its item closed, it is the prompt card gone quiet with the
// owner's answer; any other answered request is the message's quiet line.
// See docs/design.md#the-transcript and docs/model.md#owner-interaction

import HearthKit
import HearthUI
import SwiftUI

/// A request in the thread: its card while open, then its answer.
struct RequestRow: View {
    let message: Hearth_V1_Message
    let item: Hearth_V1_InboxItem?
    let inbox: InboxStore?
    let bot: Hearth_V1_Bot
    /// Whether the card's main button answers to Return.
    let isDefault: Bool

    var body: some View {
        if let item, let inbox {
            InboxItemCard(store: inbox, item: item, bot: bot, isDefault: isDefault)
        } else if Self.isAnswered(message) {
            PromptCard(message: message, bot: bot)
        } else {
            MessageRow(message: message, author: message.authorBotID.isEmpty ? nil : bot)
        }
    }

    /// Whether `message` is a question the owner has answered with its
    /// control, which the thread keeps as a quiet card.
    static func isAnswered(_ message: Hearth_V1_Message) -> Bool {
        message.kind == .question && message.hasAnswer
    }
}
