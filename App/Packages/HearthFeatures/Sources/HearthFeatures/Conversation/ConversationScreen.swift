// ConversationScreen is a direct-message conversation with a bot: the
// transcript, the run's status line and the composer. It composes MessageRow,
// ToolCallRow, RunStatusLine and Composer over the store's state; the
// transcript is a lazy stack anchored to the bottom, so the newest message
// stays in view as it streams, and reaching the top pages older messages in.
// Show in Finder reaches the revealInFinder action the app target sets.

import HearthKit
import HearthUI
import SwiftUI

/// A conversation with one bot.
public struct ConversationScreen: View {
    let store: ConversationStore
    let bot: Hearth_V1_Bot

    public init(store: ConversationStore, bot: Hearth_V1_Bot) {
        self.store = store
        self.bot = bot
    }

    public var body: some View {
        VStack(spacing: 8) {
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 12) {
                    if store.hasOlder {
                        ProgressView()
                            .frame(maxWidth: .infinity)
                            .onAppear { Task { await store.loadOlder() } }
                    }
                    ForEach(store.rows) { row in
                        switch row {
                        case .message(let message):
                            MessageRow(
                                message: message, author: message.authorBotID.isEmpty ? nil : bot)
                        case .tool(let call, _, let result):
                            ToolCallRow(call: call, result: result)
                        }
                    }
                }
                .padding()
            }
            .defaultScrollAnchor(.bottom)
            VStack(alignment: .leading, spacing: 6) {
                RunStatusLine(run: store.run, botName: bot.name)
                if let error = store.error {
                    Text(error).foregroundStyle(.primary)
                }
                Composer(
                    label: "Message \(bot.name)", isWorking: store.isWorking,
                    onSend: { text in await store.send(text) },
                    onStop: { Task { await store.cancel() } })
            }
            .padding([.horizontal, .bottom])
        }
        .navigationTitle(bot.name)
        .task(id: store.conversationID) { await store.load() }
    }
}
