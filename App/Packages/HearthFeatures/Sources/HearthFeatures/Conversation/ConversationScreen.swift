// ConversationScreen is a direct-message conversation with a bot: the
// transcript, the run's status line and the composer. It composes MessageRow,
// ToolCallRow, RunStatusLine and Composer over the store's state. The
// transcript is a lazy stack in a readable column, anchored to the bottom so
// the newest message stays in view as it streams, and reaching the top pages
// older messages in. The status line and the composer are a bar on the
// transcript's bottom edge, so messages scroll beneath the glass. A
// conversation with nothing in it says hello instead. Show in Finder reaches
// the revealInFinder action the app target sets.
// See docs/design.md#the-transcript and docs/design.md#empty-states-and-first-run

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// A conversation with one bot.
public struct ConversationScreen: View {
    let store: ConversationStore
    let bot: Hearth_V1_Bot

    /// Whether the first page has come back, so an empty transcript is known to
    /// be empty rather than not yet loaded.
    @State private var loaded = false

    public init(store: ConversationStore, bot: Hearth_V1_Bot) {
        self.store = store
        self.bot = bot
    }

    public var body: some View {
        content
            .safeAreaBar(edge: .bottom) { bar }
            .navigationTitle(bot.name)
            .task(id: store.conversationID) {
                await store.load()
                loaded = true
            }
    }

    @ViewBuilder
    private var content: some View {
        if !loaded {
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        } else if store.rows.isEmpty, let error = store.error {
            ContentUnavailableView {
                Label("Couldn't load the conversation", systemImage: "exclamationmark.triangle")
            } description: {
                Text(error)
            } actions: {
                Button("Try Again") { Task { await store.load() } }
            }
        } else if store.rows.isEmpty {
            ContentUnavailableView {
                Label {
                    Text("Say hello to \(bot.name)")
                } icon: {
                    Avatar(bot: bot, size: .large)
                }
            } description: {
                Text(
                    "\(bot.name) is your \(bot.role). Ask for something and watch the work happen here."
                )
            }
        } else {
            transcript
        }
    }

    private var transcript: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: Theme.Spacing.lg) {
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
            .frame(maxWidth: Theme.Size.readableWidth)
            .padding(.horizontal, Theme.Spacing.lg)
            .padding(.vertical, Theme.Spacing.lg)
            .frame(maxWidth: .infinity)
        }
        .defaultScrollAnchor(.bottom)
    }

    private var bar: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            RunStatusLine(run: store.run, botName: bot.name)
                .padding(.horizontal, Theme.Spacing.lg)
            if let error = store.error, loaded {
                Text(error)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(Tone.negative))
                    .padding(.horizontal, Theme.Spacing.lg)
            }
            Composer(
                label: "Message \(bot.name)", isWorking: store.isWorking,
                onSend: { text in await store.send(text) },
                onStop: { Task { await store.cancel() } })
        }
        .frame(maxWidth: Theme.Size.readableWidth)
        .frame(maxWidth: .infinity)
    }
}
