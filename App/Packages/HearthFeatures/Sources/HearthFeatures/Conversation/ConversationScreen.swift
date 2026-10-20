// ConversationScreen is a direct-message conversation with a bot: the
// transcript, the run's status line and the composer. It composes MessageRow,
// ToolCallRow, RunStatusLine and Composer over the store's state. The
// transcript is a lazy stack in a readable column, anchored to the bottom so
// the newest message stays in view as it streams, and reaching the top pages
// older messages in. The status line and the composer are a bar on the
// transcript's bottom edge, so messages scroll beneath the glass. A
// conversation with nothing in it shows the bot's introduction card, centred,
// and a bot without one says hello instead; once it has messages the card
// collapses to an "About <name>" row pinned under the title, which opens into
// the same card, and a chip on the card fills the composer without sending.
// Show in Finder reaches the revealInFinder action the app target sets. Only the owner's own send
// moves in; everything from the core appears in place, and the empty state
// fades out. An approval or a question the run parked on stands in the thread
// as a RequestRow: its Inbox card while it is open, then a quiet line, or a
// quiet card for a question answered with its control. While it is open the
// status line says the bot is waiting and the composer asks for the answer
// instead of a message. A run that failed or was stopped offers Retry on the
// status line. When the core has written a summary of the earlier part, a
// disclosure row above the oldest loaded message opens it in secondary text;
// the composer takes files dropped on it as chips, and the store sends them
// as paths.
// See docs/design.md#the-transcript, docs/design.md#empty-states-and-first-run,
// docs/design.md#motion, docs/model.md#owner-interaction and
// docs/model.md#9-failure-handling

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
    @State private var summaryOpen: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A screen for `store`; `summaryOpen` starts the earlier-summary row open.
    public init(store: ConversationStore, bot: Hearth_V1_Bot, summaryOpen: Bool = false) {
        self.store = store
        self.bot = bot
        _summaryOpen = State(initialValue: summaryOpen)
    }

    public var body: some View {
        content
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: store.rows.isEmpty
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .safeAreaBar(edge: .bottom) { bar }
            .safeAreaBar(edge: .top) { aboutBar }
            .navigationTitle(bot.name)
            .task(id: store.conversationID) {
                await store.load()
                loaded = true
            }
    }

    /// Whether the core sent the bot an introduction to show.
    private var hasIntroduction: Bool { !bot.introduction.purpose.isEmpty }

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
        } else if store.rows.isEmpty, hasIntroduction {
            // Centred while it fits; a short window scrolls it from the top.
            ViewThatFits(in: .vertical) {
                introduction
                ScrollView { introduction }
            }
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .transition(.opacity)
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

    private var introduction: some View {
        IntroductionCard(introduction: bot.introduction, bot: bot, onTry: store.fill)
            .padding(Theme.Spacing.lg)
            .frame(maxWidth: .infinity)
    }

    /// The header pinned under the title once there is something to scroll
    /// past: nothing while the card is the empty state.
    @ViewBuilder
    private var aboutBar: some View {
        if loaded, !store.rows.isEmpty, hasIntroduction {
            AboutRow(
                isExpanded: Bindable(store).aboutOpen, introduction: bot.introduction, bot: bot,
                onTry: store.fill
            )
            .frame(maxWidth: Theme.Size.readableWidth)
            .frame(maxWidth: .infinity)
            .padding(.horizontal, Theme.Spacing.lg)
            .padding(.top, Theme.Spacing.sm)
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
                if !store.summary.isEmpty {
                    summary
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

    private var summary: some View {
        DisclosureRow(
            isExpanded: $summaryOpen,
            label: {
                VStack(alignment: .leading, spacing: 0) {
                    Text("Earlier in this conversation")
                    Text("What \(bot.name) remembers of the earlier part")
                        .font(Theme.caption)
                }
            },
            content: {
                Text(store.summary)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(Theme.Spacing.md)
                    .background(
                        RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                            .fill(Theme.color(.surfaceSecondary))
                    )
            })
    }

    /// The owner's own message, which moves in; a system notice is the core's.
    private static func isSend(_ message: Hearth_V1_Message) -> Bool {
        message.authorBotID.isEmpty && message.kind == .text
    }

    private static let sendTransition = AnyTransition.move(edge: .bottom)
        .combined(with: .opacity)

    private var bar: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            RunStatusLine(
                run: store.run, botName: bot.name, isWaiting: store.isWaiting,
                action: store.canRetry
                    ? .init("Retry") { Task { await store.retry() } } : nil
            )
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
                waitingPrompt: store.waitingPrompt(botName: bot.name)
                    ?? store.pausedPrompt(botName: bot.name),
                attachments: store.attachments,
                attachmentNotice: store.attachmentNotice,
                onDrop: { urls in store.attach(urls) },
                onRemove: { file in store.removeAttachment(path: file.path) },
                text: Bindable(store).draft, focusToken: store.focusComposer,
                onSend: { text in await store.send(text) },
                onStop: { Task { await store.cancel() } })
        }
        .frame(maxWidth: Theme.Size.readableWidth)
        .frame(maxWidth: .infinity)
    }
}
