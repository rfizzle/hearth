// ConversationStore holds one conversation's transcript as the core last sent
// it, and its run. It loads the newest page, pages older ones in on request,
// and applies what the relay delivers: a message delta appends to the cached
// message, an upsert replaces it whole (the core sends the final text as one),
// a run change moves the status line, and a resync refetches the page and
// forgets the run until the core reports it again. Send
// and Stop are calls; the store decides nothing about either.
// See docs/architecture.md#3-the-contract-sketch

import Foundation
import HearthKit
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

        public var id: String {
            switch self {
            case .message(let message): message.id
            case .tool(_, let id, _): id
            }
        }
    }

    /// The conversation this store follows.
    public let conversationID: String
    /// The loaded messages, ascending by seq.
    public private(set) var messages: [Hearth_V1_Message] = []
    /// Whether the core holds messages older than the first loaded.
    public private(set) var hasOlder = false
    /// The conversation's latest run, once the core has reported one.
    public private(set) var run: Hearth_V1_Run?
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    private var loadingOlder = false
    private let client: any HearthClient
    private let pageSize: Int32
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store for `conversationID` over `client`, fed by `relay`; a
    /// `pageSize` of 0 asks for the core's default page.
    public init(
        client: any HearthClient, relay: EventRelay, conversationID: String, pageSize: Int32 = 0
    ) {
        self.client = client
        self.relay = relay
        self.conversationID = conversationID
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

    /// The transcript as rows: each tool call carries its result, matched by
    /// call id, and a result whose call is not loaded stands alone.
    public var rows: [Row] {
        let results = Dictionary(
            messages.filter { $0.kind == .toolResult }.map { ($0.tool.callID, $0) },
            uniquingKeysWith: { _, last in last })
        let calls = Set(messages.filter { $0.kind == .toolCall }.map(\.tool.callID))
        return messages.compactMap { message in
            switch message.kind {
            case .toolCall:
                .tool(call: message.tool, id: message.id, result: results[message.tool.callID])
            case .toolResult:
                calls.contains(message.tool.callID)
                    ? nil : .tool(call: message.tool, id: message.id, result: message)
            default:
                .message(message)
            }
        }
    }

    /// Fetches the newest page, replacing what is loaded.
    public func load() async {
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

    /// Sends the owner's `text`, which starts a run; false when the core
    /// refused it.
    @discardableResult
    public func send(_ text: String) async -> Bool {
        do {
            let response = try await client.send(conversationID: conversationID, text: text)
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
        case .resync:
            // The run's last change may be among the events the core no longer
            // holds, and no call fetches a run; the next change or send sets it.
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
