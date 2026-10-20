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
// The window is titled by the bot's name, or by the title given.
// A project's conversation is the same screen with the ProjectHeader pinned
// above the transcript, the Chief of Staff answering, the project's name as
// the title and its light as the subtitle; it has no introduction card, an
// empty one says nothing is here yet, and with no Chief of Staff the composer
// is replaced by the invitation to hire one. A proposal the Chief of Staff
// made in a conversation stands in its bottom bar as a ProposalCard above the
// status line.
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
    let title: String?
    let project: Hearth_V1_Project?
    let hasAnswerer: Bool
    let proposals: [Hearth_V1_Project]
    let onCreateProposal: (Hearth_V1_Project) -> Void
    let onDismissProposal: (Hearth_V1_Project) -> Void
    let onMarkDone: () -> Void
    let onHire: () -> Void

    /// Whether the first page has come back, so an empty transcript is known to
    /// be empty rather than not yet loaded.
    @State private var loaded = false
    @State private var summaryOpen: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A screen for `store`; `summaryOpen` starts the earlier-summary row open,
    /// and `title` names the window instead of the bot.
    public init(
        store: ConversationStore, bot: Hearth_V1_Bot, title: String? = nil,
        summaryOpen: Bool = false, proposals: [Hearth_V1_Project] = [],
        onCreateProposal: @escaping (Hearth_V1_Project) -> Void = { _ in },
        onDismissProposal: @escaping (Hearth_V1_Project) -> Void = { _ in }
    ) {
        self.store = store
        self.bot = bot
        self.title = title
        project = nil
        hasAnswerer = true
        self.proposals = proposals
        self.onCreateProposal = onCreateProposal
        self.onDismissProposal = onDismissProposal
        onMarkDone = {}
        onHire = {}
        _summaryOpen = State(initialValue: summaryOpen)
    }

    /// A screen for the conversation of `project`, answered by `coordinator`;
    /// with none, the composer gives way to the invitation to hire someone.
    public init(
        store: ConversationStore, project: Hearth_V1_Project, coordinator: Hearth_V1_Bot?,
        proposals: [Hearth_V1_Project] = [],
        onCreateProposal: @escaping (Hearth_V1_Project) -> Void = { _ in },
        onDismissProposal: @escaping (Hearth_V1_Project) -> Void = { _ in },
        onMarkDone: @escaping () -> Void = {}, onHire: @escaping () -> Void = {}
    ) {
        self.store = store
        bot = coordinator ?? Hearth_V1_Bot()
        title = nil
        self.project = project
        hasAnswerer = coordinator != nil
        self.proposals = proposals
        self.onCreateProposal = onCreateProposal
        self.onDismissProposal = onDismissProposal
        self.onMarkDone = onMarkDone
        self.onHire = onHire
        _summaryOpen = State(initialValue: false)
    }

    public var body: some View {
        content
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: store.rows.isEmpty
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
            .safeAreaBar(edge: .bottom) { bar }
            .safeAreaBar(edge: .top) { aboutBar }
            .safeAreaInset(edge: .top, spacing: 0) { projectBar }
            .navigationTitle(project?.name ?? title ?? bot.name)
            .modifier(LightSubtitle(text: project?.status.lightLabel))
            .task(id: store.conversationID) {
                await store.load()
                loaded = true
            }
    }

    /// Whether the core sent the bot an introduction to show.
    private var hasIntroduction: Bool { project == nil && !bot.introduction.purpose.isEmpty }

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
        } else if store.rows.isEmpty, project != nil {
            if hasAnswerer {
                ContentUnavailableView {
                    Label("Nothing here yet", systemImage: "folder")
                } description: {
                    Text("Tell \(bot.name) what to do first.")
                }
                .transition(.opacity)
            } else {
                // The bar's invitation says it; nothing else would.
                Color.clear
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

    /// The project's brief, pinned above the transcript.
    @ViewBuilder
    private var projectBar: some View {
        if let project {
            ProjectHeader(project: project, onMarkDone: onMarkDone)
                .frame(maxWidth: Theme.Size.readableWidth)
                .frame(maxWidth: .infinity)
                .padding(.horizontal, Theme.Spacing.lg)
                .padding(.vertical, Theme.Spacing.sm)
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

    /// The owner's own message, which moves in; a system notice is the core's.
    private static func isSend(_ message: Hearth_V1_Message) -> Bool {
        message.authorBotID.isEmpty && message.kind == .text
    }

    private static let sendTransition = AnyTransition.move(edge: .bottom)
        .combined(with: .opacity)

    /// The proposals made in this conversation, newest last.
    private var openProposals: [Hearth_V1_Project] {
        proposals.filter { $0.proposedInConversationID == store.conversationID }
    }

    @ViewBuilder
    private var bar: some View {
        if hasAnswerer { answeredBar } else { unansweredBar }
    }

    /// With no one to answer a project, the invitation stands where the
    /// composer would.
    private var unansweredBar: some View {
        ContentUnavailableView {
            Label("No one answers projects yet", systemImage: "folder")
        } description: {
            Text("Hire a Chief of Staff and this conversation wakes up.")
        } actions: {
            Button("Hire Someone…", action: onHire)
        }
        .frame(maxWidth: Theme.Size.readableWidth)
        .frame(maxWidth: .infinity)
    }

    private var answeredBar: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            ForEach(openProposals, id: \.id) { proposal in
                ProposalCard(
                    project: proposal, onCreate: { onCreateProposal(proposal) },
                    onDismiss: { onDismissProposal(proposal) }
                )
                .padding(.horizontal, Theme.Spacing.lg)
            }
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

extension ConversationScreen {
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
}

/// The window's subtitle, set only when there is one so a bot's role set
/// outside the screen stays.
private struct LightSubtitle: ViewModifier {
    let text: String?

    @ViewBuilder
    func body(content: Content) -> some View {
        if let text {
            content.navigationSubtitle(text)
        } else {
            content
        }
    }
}
