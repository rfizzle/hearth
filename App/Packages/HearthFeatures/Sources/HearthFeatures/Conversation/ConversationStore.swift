// ConversationStore holds one conversation's transcript as the core last sent
// it, and its run. It loads the newest page, pages older ones in on request,
// and applies what the relay delivers: a message delta appends to the cached
// message, an upsert replaces it whole (the core sends the final text as one),
// a run change moves the status line, a deleted message leaves the list, and
// a resync refetches the conversation and the page. Loading asks for the
// conversation first and takes its latest run, so a screen opened mid-run
// shows the work at once. Send, Stop and Retry are calls; the store decides
// nothing about any of them.
// See docs/architecture.md#3-the-contract-sketch
//
// The composer's draft lives here, so a chip on the bot's introduction can
// fill it, and so does whether the "About" header is open.
//
// An approval request or a question in the thread is joined by message id to
// the Inbox's open items, so while one is open the thread draws its card; the
// conversation then waits for the owner and Send is held, since the core
// refuses a message while a run waits on an answer.
// See docs/model.md#8-approvals

import Foundation
import HearthKit
import HearthUI
import Observation

/// The store behind a conversation screen.
@MainActor
@Observable
public final class ConversationStore {
    /// One row of the transcript as the screen draws it.
    public enum Row: Identifiable, Equatable {
        /// A text or system message.
        case message(Hearth_V1_Message)
        /// A tool call and, once it has arrived, its result.
        case tool(call: Hearth_V1_ToolCall, id: String, result: Hearth_V1_Message?)
        /// An approval request or a question, with its Inbox item while that
        /// is open.
        case request(Hearth_V1_Message, item: Hearth_V1_InboxItem?)
        /// A finished run's calls and interim texts folded into one line;
        /// `duration` is nil when the run's times are not known.
        case worked(id: String, duration: TimeInterval?, steps: [TraceStep])

        public var id: String {
            switch self {
            case .message(let message): message.id
            case .tool(_, let id, _): id
            case .request(let message, _): message.id
            case .worked(let id, _, _): id
            }
        }
    }

    /// The conversation this store follows.
    public let conversationID: String
    /// What waits on the owner, joined to the thread; nil draws no cards.
    public let inbox: InboxStore?
    /// Where paused is read from, with `botID`; nil never pauses.
    public let usage: UsageStore?
    /// The bot whose conversation this is, for the pause.
    public let botID: String?
    /// The loaded messages, ascending by seq.
    public private(set) var messages: [Hearth_V1_Message] = []
    /// Whether the core holds messages older than the first loaded.
    public private(set) var hasOlder = false
    /// The conversation's latest run, once the core has reported one.
    public private(set) var run: Hearth_V1_Run?
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// What the bot remembers of the part of the conversation before the
    /// loaded page; empty until the core has written one.
    public private(set) var summary = ""
    /// The files dropped on the composer and not yet sent, in the order they
    /// arrived. A file is its resolved path and name; the app reads nothing
    /// from it.
    public private(set) var attachments: [Hearth_V1_FileRef] = []
    /// Why the last drop took nothing, in a line, or nil.
    public private(set) var attachmentNotice: String?
    /// What is typed in the composer and not yet sent. The store owns it so a
    /// suggestion chip elsewhere on the screen can fill it.
    public var draft = ""
    /// Bumped each time the composer should take focus, which a chip's fill
    /// asks for; the screen watches it.
    public private(set) var focusComposer = 0
    /// Whether the "About" header is open. It starts closed and is not kept
    /// across launches.
    public var aboutOpen = false

    private var loadingOlder = false
    private let client: any HearthClient
    private let pageSize: Int32
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store for `conversationID` over `client`, fed by `relay`, with the
    /// open items from `inbox`; a `pageSize` of 0 asks for the core's default
    /// page.
    public init(
        client: any HearthClient, relay: EventRelay, conversationID: String,
        inbox: InboxStore? = nil, usage: UsageStore? = nil, botID: String? = nil,
        pageSize: Int32 = 0
    ) {
        self.usage = usage
        self.botID = botID
        self.client = client
        self.relay = relay
        self.conversationID = conversationID
        self.inbox = inbox
        self.pageSize = pageSize
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// Whether the run has not ended: Send waits and Stop is offered.
    public var isWorking: Bool {
        switch run?.state {
        case .queued, .working, .waitingOwner: true
        default: false
        }
    }

    /// The open approval or question the run waits on, if there is one.
    public var waitingItem: Hearth_V1_InboxItem? {
        inbox?.waitingItem(inConversation: conversationID)
    }

    /// Whether the conversation waits for the owner: an item is open, or the
    /// core says the run waits.
    public var isWaiting: Bool {
        waitingItem != nil || run?.state == .waitingOwner
    }

    /// What the composer says in place of a message while the conversation
    /// waits for the owner, or nil when it does not.
    public func waitingPrompt(botName: String) -> String? {
        isWaiting ? "Answer \(botName) above first" : nil
    }

    /// Whether the bot is paused this month, by its budget or the team's.
    public var isPaused: Bool {
        botID.map { usage?.isPaused($0) ?? false } ?? false
    }

    /// What the composer says in place of a message while the bot is paused,
    /// or nil when it is not: Send is off until the owner resumes.
    public func pausedPrompt(botName: String) -> String? {
        isPaused ? "\(botName) is paused" : nil
    }

    /// The transcript as rows: each tool call carries its result, a request
    /// its item while that is open, and a finished run is folded into one
    /// worked row with its last text left outside as the answer.
    public var rows: [Row] {
        ConversationRows.build(messages: messages, run: run) {
            inbox?.openItem(forMessage: $0)
        }
    }

    /// Whether the latest run ended in a way Retry can run again.
    public var canRetry: Bool {
        run?.state == .failed || run?.state == .cancelled
    }

    /// Fetches the conversation, for its latest run, then the newest page,
    /// replacing what is loaded. A conversation that cannot be fetched leaves
    /// the run as it was; the page decides whether the call failed.
    public func load() async {
        let conversation = try? await client.getConversation(id: conversationID)
        if let conversation {
            summary = conversation.summary
            if conversation.hasLatestRun, run == nil {
                run = conversation.latestRun
            }
        }
        do {
            let page = try await client.listMessages(
                conversationID: conversationID, beforeSeq: 0, limit: pageSize)
            messages = page.messages
            hasOlder = page.hasMore_p
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Fetches the page before the first loaded message.
    public func loadOlder() async {
        guard hasOlder, !loadingOlder, let first = messages.first else { return }
        loadingOlder = true
        defer { loadingOlder = false }
        do {
            let page = try await client.listMessages(
                conversationID: conversationID, beforeSeq: first.seq, limit: pageSize)
            let loaded = Set(messages.map(\.id))
            messages.insert(contentsOf: page.messages.filter { !loaded.contains($0.id) }, at: 0)
            hasOlder = page.hasMore_p
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Puts `text` in the composer, replacing what was there, and asks for
    /// focus there. It never sends: the owner reads it, changes it, and sends.
    public func fill(_ text: String) {
        draft = text
        focusComposer += 1
    }

    /// Takes the files dropped on the composer. Only file URLs count; each is
    /// sent as its standardised path with symlinks resolved, which is
    /// the one thing the app learns of it besides whether it is a folder.
    /// A folder is refused with a caption, a file already attached is kept
    /// once, and a path that does not exist is left for the core to refuse.
    public func attach(_ urls: [URL]) {
        attachmentNotice = nil
        for url in urls where url.isFileURL {
            let resolved = url.standardizedFileURL.resolvingSymlinksInPath()
            var isDirectory: ObjCBool = false
            let exists = FileManager.default.fileExists(
                atPath: resolved.path, isDirectory: &isDirectory)
            if exists && isDirectory.boolValue {
                attachmentNotice = "\(resolved.lastPathComponent) is a folder; drop a file instead."
                continue
            }
            guard !attachments.contains(where: { $0.path == resolved.path }) else { continue }
            attachments.append(
                .with {
                    $0.path = resolved.path
                    $0.name = resolved.lastPathComponent
                })
        }
    }

    /// Drops one attached file before it is sent.
    public func removeAttachment(path: String) {
        attachments.removeAll { $0.path == path }
        attachmentNotice = nil
    }

    /// Sends the owner's `text` with the attached files, which starts a run;
    /// false when the core refused it, and then the files stay attached.
    @discardableResult
    public func send(_ text: String) async -> Bool {
        let sent = attachments
        do {
            let response = try await client.send(
                conversationID: conversationID, text: text, attachmentPaths: sent.map(\.path))
            attachments.removeAll { file in sent.contains { $0.path == file.path } }
            attachmentNotice = nil
            upsert(response.message)
            // The run's own events can arrive before this answer does; theirs
            // are newer, so the answer only introduces a run.
            if run?.id != response.run.id {
                run = response.run
            }
            error = nil
            return true
        } catch {
            self.error = error.localizedDescription
            return false
        }
    }

    /// Stops the working run, keeping what the bot wrote.
    public func cancel() async {
        guard let run, isWorking else { return }
        do {
            try await client.cancel(runID: run.id)
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Runs the failed or stopped run again; the core's answer is the new run,
    /// and its events follow.
    public func retry() async {
        guard let run, canRetry else { return }
        do {
            let next = try await client.retry(runID: run.id)
            // The run's own events can arrive first; theirs are newer.
            if self.run?.id != next.id {
                self.run = next
            }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .messageDelta(let delta) where delta.conversationID == conversationID:
            if let index = messages.firstIndex(where: { $0.id == delta.messageID }) {
                messages[index].content += delta.text
            }
        case .messageUpserted(let change) where change.message.conversationID == conversationID:
            upsert(change.message)
        case .runChanged(let change) where change.run.conversationID == conversationID:
            run = change.run
        case .conversationChanged(let change) where change.conversation.id == conversationID:
            summary = change.conversation.summary
        case .messageDeleted(let deleted) where deleted.conversationID == conversationID:
            messages.removeAll { $0.id == deleted.messageID }
        case .resync:
            // The run's last change may be among the events the core no longer
            // holds; the conversation says where its latest run stands.
            run = nil
            await load()
        default:
            break
        }
    }

    private func upsert(_ message: Hearth_V1_Message) {
        if let index = messages.firstIndex(where: { $0.id == message.id }) {
            messages[index] = message
        } else if let first = messages.first, hasOlder, message.seq < first.seq {
            // Older than the loaded page: paging brings it in.
            return
        } else {
            let index = messages.firstIndex { $0.seq > message.seq } ?? messages.endIndex
            messages.insert(message, at: index)
        }
    }
}
