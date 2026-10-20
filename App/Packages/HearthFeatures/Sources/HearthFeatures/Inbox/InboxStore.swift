// InboxStore holds what waits on the owner as the core last listed it: the
// open items, newest first, and for each bot with an approval its grants, so a
// card offers Always Allow only where the core says the capability can stand.
// It fetches the list, applies each inbox change the relay delivers (an open
// item joins, a closed one leaves), keeps grants current, and refetches on a
// resync. Decide, answer, acknowledge and retry are calls; while one is in
// flight its card waits, and once the core answers the card shows the core's
// outcome until the item's closing change removes it. A question's answer may
// be changed until then, since the core holds it open for a short grace. The
// store decides nothing about any of them. The sidebar's count, the Inbox screen and the
// conversation's cards read it.
// See docs/model.md#owner-interaction and docs/model.md#8-approvals

import Foundation
import HearthDesign
import HearthKit
import HearthUI
import Observation

/// The store behind the Inbox.
@MainActor
@Observable
public final class InboxStore {
    /// The Inbox's two groups.
    public enum Section: String, CaseIterable, Identifiable, Sendable {
        /// Approvals and questions: a run waits on them.
        case needsDecision = "Needs a decision"
        /// Failures and budgets: nothing waits, the owner notes them.
        case toNote = "To note"

        public var id: String { rawValue }

        /// Whether an item of `kind` belongs here.
        func holds(_ kind: Hearth_V1_InboxKind) -> Bool {
            switch kind {
            case .approval, .question: self == .needsDecision
            default: self == .toNote
            }
        }
    }

    /// The open items, newest first.
    public private(set) var items: [Hearth_V1_InboxItem] = []
    /// Whether the first list has come back.
    public private(set) var loaded = false
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    /// Items with a call in flight.
    private(set) var pending: Set<String> = []
    /// The core's answer for an item still listed, until its change arrives.
    private(set) var answered: [String: Hearth_V1_InboxItem] = [:]
    /// Each bot's grants, for the bots with an approval.
    private var grants: [String: [Hearth_V1_Grant]] = [:]

    private let client: any HearthClient
    /// The usage a Resume answers into, so the sidebar and the composer see
    /// the bot work again at once.
    private let usage: UsageStore?
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store over `client`, fed by `relay`; `usage` hears the summary a
    /// Resume brings back.
    public init(client: any HearthClient, relay: EventRelay, usage: UsageStore? = nil) {
        self.client = client
        self.relay = relay
        self.usage = usage
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// How many items are open: the sidebar's count.
    public var openCount: Int {
        items.count
    }

    /// The open items in `section`, newest first.
    public func items(in section: Section) -> [Hearth_V1_InboxItem] {
        items.filter { section.holds($0.kind) }
    }

    /// The sections that hold anything, in order.
    public var sections: [Section] {
        Section.allCases.filter { !items(in: $0).isEmpty }
    }

    /// The open item standing behind the thread message `messageID`.
    public func openItem(forMessage messageID: String) -> Hearth_V1_InboxItem? {
        items.first { $0.messageID == messageID }
    }

    /// The newest open approval or question in `conversationID`: what its run
    /// waits on.
    public func waitingItem(inConversation conversationID: String) -> Hearth_V1_InboxItem? {
        items.first {
            $0.conversationID == conversationID && Section.needsDecision.holds($0.kind)
        }
    }

    /// Where `item`'s card is.
    public func state(of item: Hearth_V1_InboxItem) -> InboxCardState {
        // A changed answer waits like the first one did.
        if pending.contains(item.id) { return .deciding }
        if let closed = answered[item.id] {
            return .closed(Self.outcome(closed), Self.tone(closed))
        }
        return .open
    }

    /// Whether `item`'s card offers Always Allow: every grant behind its
    /// call's capability says it can stand.
    public func canAlwaysAllow(_ item: Hearth_V1_InboxItem) -> Bool {
        guard item.kind == .approval, let capability = Self.capability(forTool: item.call.name)
        else { return false }
        let behind = (grants[item.botID] ?? []).filter { $0.capability == capability }
        return !behind.isEmpty && behind.allSatisfy(\.canAllow)
    }

    /// Fetches the open items, and the grants of each bot with an approval.
    public func load() async {
        do {
            let fetched = try await client.listInbox(includeClosed: false)
            items = Self.newestFirst(fetched.filter { $0.state == .open })
            answered = answered.filter { id, _ in items.contains { $0.id == id } }
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
            return
        }
        await loadGrants()
    }

    /// Answers the approval `item` with `decision`; `reason` goes with a deny.
    public func decide(
        _ item: Hearth_V1_InboxItem, _ decision: Hearth_V1_Decision, reason: String = ""
    ) async {
        await call(item) {
            try await self.client.decide(itemID: item.id, decision, reason: reason).item
        }
    }

    /// Answers the question `itemID` with the owner's words `text` and, when
    /// it has a control, the control's answer `value`. Until the core closes
    /// the item a question can be answered again: the core holds an answer
    /// open for a short grace before the run goes on, and the newer answer
    /// replaces it.
    /// See docs/model.md#owner-interaction
    public func answer(itemID: String, text: String, value: Hearth_V1_WidgetAnswer? = nil) async {
        guard let item = items.first(where: { $0.id == itemID }) else { return }
        await call(item) {
            try await self.client.answer(itemID: itemID, text, value: value).item
        }
    }

    /// Whether `item`'s answered card offers Change: the core has the answer
    /// and has not closed the item, so a new answer still replaces it.
    public func canChange(_ item: Hearth_V1_InboxItem) -> Bool {
        item.kind == .question && answered[item.id] != nil && !pending.contains(item.id)
            && items.contains { $0.id == item.id }
    }

    /// The item as the core answered it, while the card still shows it.
    public func answeredItem(_ item: Hearth_V1_InboxItem) -> Hearth_V1_InboxItem? {
        answered[item.id]
    }

    /// Closes the notice `item`.
    public func acknowledge(_ item: Hearth_V1_InboxItem) async {
        await call(item) { try await self.client.acknowledge(itemID: item.id) }
    }

    /// Whether `item` is the pause notice, which Resume answers: the core
    /// starts its effect with "Paused: ". The 80% warning is only noted.
    public func canResume(_ item: Hearth_V1_InboxItem) -> Bool {
        item.kind == .budget && item.effect.hasPrefix("Paused: ")
    }

    /// Lets the paused bot behind `item` (the whole team, when the item names
    /// no bot) work again; the core closes the item.
    public func resume(_ item: Hearth_V1_InboxItem) async {
        guard canResume(item) else { return }
        await call(item) {
            let summary = try await self.client.resume(botID: item.botID)
            self.usage?.apply(summary)
            var resumed = item
            resumed.state = .closed
            resumed.outcome = "Resumed"
            return resumed
        }
    }

    /// Runs the failed run behind `item` again; the core closes the item.
    public func retry(_ item: Hearth_V1_InboxItem) async {
        await call(item) {
            _ = try await self.client.retry(runID: item.runID)
            var retried = item
            retried.state = .closed
            retried.outcome = "Trying again"
            return retried
        }
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .inboxChanged(let change):
            apply(change.item)
            // A bot's first approval needs its grants; known ones are kept.
            if change.item.state == .open { await loadGrants() }
        case .grantsChanged(let change):
            grants[change.botID] = change.grants
        case .resync:
            // A grants change may be among the events the core no longer holds.
            grants = [:]
            await load()
        default:
            break
        }
    }

    private func apply(_ item: Hearth_V1_InboxItem) {
        items.removeAll { $0.id == item.id }
        if item.state == .open {
            items = Self.newestFirst(items + [item])
        } else {
            answered[item.id] = nil
            pending.remove(item.id)
        }
    }

    /// Runs one call for `item`: the card waits while it is in flight and
    /// shows the core's answer once it comes back.
    private func call(
        _ item: Hearth_V1_InboxItem, _ body: () async throws -> Hearth_V1_InboxItem
    ) async {
        // A question answered but not yet closed takes a changed answer.
        guard items.contains(where: { $0.id == item.id }), !pending.contains(item.id),
            answered[item.id] == nil || item.kind == .question
        else { return }
        pending.insert(item.id)
        defer { pending.remove(item.id) }
        do {
            let closed = try await body()
            // The closing change may have beaten the answer here.
            if items.contains(where: { $0.id == item.id }) {
                answered[item.id] = closed
            }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func loadGrants() async {
        let bots = Set(items.filter { $0.kind == .approval }.map(\.botID))
        for bot in bots where grants[bot] == nil {
            // Without them the card offers no Always Allow, the safe side.
            grants[bot] = try? await client.listGrants(botID: bot)
        }
    }

    private static func newestFirst(_ items: [Hearth_V1_InboxItem]) -> [Hearth_V1_InboxItem] {
        items.sorted { $0.createdAt.date > $1.createdAt.date }
    }

    /// The outcome's tone: positive when the run goes on with what the owner
    /// allowed or said, quiet for a deny or a notice.
    static func tone(_ item: Hearth_V1_InboxItem) -> Tone {
        item.decision == .deny || !Section.needsDecision.holds(item.kind) ? .neutral : .positive
    }

    /// What a closed card says: the core's outcome, else that it is done.
    static func outcome(_ item: Hearth_V1_InboxItem) -> String {
        item.outcome.isEmpty ? "Done" : item.outcome
    }

    /// The capability a tool's calls are granted under, as the Access sheet
    /// names them; nil for a tool that has no standing grant.
    static func capability(forTool name: String) -> Hearth_V1_Capability? {
        switch name {
        case "files.read": .filesRead
        case "files.write": .filesWrite
        case "files.delete": .filesDelete
        case "web.fetch": .webFetch
        case "web.search": .webSearch
        default: nil
        }
    }
}
