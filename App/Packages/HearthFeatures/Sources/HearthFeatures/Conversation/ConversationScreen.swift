// ConversationScreen is a direct-message conversation with a bot: the
// transcript, the run's status line and the composer. It composes MessageRow,
// ToolCallRow, RunStatusLine and Composer over the store's state. The
// transcript is a lazy stack in a readable column, anchored to the bottom so
// the newest message stays in view as it streams, and reaching the top pages
// older messages in. The status line and the composer are a bar on the
// transcript's bottom edge, so messages scroll beneath the glass. A
// conversation with nothing in it says hello instead. Show in Finder reaches
// the revealInFinder action the app target sets. Only the owner's own send
// moves in; everything from the core appears in place, and the empty state
// fades out. An approval or a question the run parked on stands in the thread
// as a RequestRow: its Inbox card while it is open, then a quiet line, or a
// quiet card for a question answered with its control. While it is open the
// status line says the bot is waiting and the composer asks for the answer
// instead of a message.
// See docs/design.md#the-transcript, docs/design.md#empty-states-and-first-run,
// docs/design.md#motion and docs/model.md#owner-interaction

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
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(store: ConversationStore, bot: Hearth_V1_Bot) {
        self.store = store
        self.bot = bot
    }

    public var body: some View {
        content
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: store.rows.isEmpty
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
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
            .transition(.opacity)
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
                            message: message, author: message.authorBotID.isEmpty ? nil : bot
                        )
                        .transition(Self.isSend(message) ? Self.sendTransition : .identity)
                    case .tool(let call, _, let result):
                        ToolCallRow(call: call, result: result)
                            .transition(.identity)
                    case .request(let message, let item):
                        RequestRow(
                            message: message, item: item, inbox: store.inbox, bot: bot,
                            isDefault: item != nil && item?.id == store.waitingItem?.id)
                    }
                }
            }
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: store.rows.last?.id
            )
            .frame(maxWidth: Theme.Size.readableWidth)
            .padding(.horizontal, Theme.Spacing.lg)
            .padding(.vertical, Theme.Spacing.lg)
            .frame(maxWidth: .infinity)
        }
        .defaultScrollAnchor(.bottom)
        .defaultScrollAnchor(.bottom, for: .sizeChanges)
    }

    /// The owner's own message, which moves in; a system notice is the core's.
    private static func isSend(_ message: Hearth_V1_Message) -> Bool {
        message.authorBotID.isEmpty && message.kind == .text
    }

    private static let sendTransition = AnyTransition.move(edge: .bottom)
        .combined(with: .opacity)

    private var bar: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            RunStatusLine(run: store.run, botName: bot.name, isWaiting: store.isWaiting)
                .padding(.horizontal, Theme.Spacing.lg)
            if let error = store.error, loaded {
                Text(error)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(Tone.negative))
                    .padding(.horizontal, Theme.Spacing.lg)
            }
            Composer(
                label: "Message \(bot.name)",
                isWorking: store.isWorking && !store.isWaiting,
                waitingPrompt: store.waitingPrompt(botName: bot.name),
                onSend: { text in await store.send(text) },
                onStop: { Task { await store.cancel() } })
        }
        .frame(maxWidth: Theme.Size.readableWidth)
        .frame(maxWidth: .infinity)
    }
}
