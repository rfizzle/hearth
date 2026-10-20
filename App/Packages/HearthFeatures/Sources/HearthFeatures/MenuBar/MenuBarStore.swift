// MenuBarStore is what the menu bar extra shows, read from the session's own
// stores rather than a second copy: who is working comes from each bot's
// latest run as the bot list keeps it from the relay's run changes, and what
// needs the owner is the Inbox's open items, so the count is the sidebar's
// count. A failed run is its failure item until Retry or acknowledge, a run
// waiting on the owner is its approval or question item, and a budget pause is
// its item; nothing is counted twice. A resync refetches both lists through
// those stores, so the extra follows. The quick ask sends through the bot's
// own conversation store, the one the window opens, and its reply is that
// conversation's newest two messages. The store decides nothing.
// See docs/model.md#owner-interaction and docs/design.md#window-anatomy

import HearthKit
import HearthUI
import Observation

/// The store behind the menu bar extra and the quick ask.
@MainActor
@Observable
public final class MenuBarStore {
    /// The status item's state: rest, someone working, something waiting,
    /// or both.
    public struct Glyph: Hashable, Sendable {
        /// Whether any bot's run works.
        public let working: Bool
        /// Whether anything waits on the owner.
        public let needsYou: Bool

        /// A glyph for those two facts.
        public init(working: Bool, needsYou: Bool) {
            self.working = working
            self.needsYou = needsYou
        }
    }

    /// The bots and their latest runs.
    public let bots: BotListStore
    /// What waits on the owner.
    public let inbox: InboxStore
    /// What the owner is typing into the quick ask.
    public var draft = ""
    /// Who the owner chose to ask, if anyone; the view remembers it.
    public var chosenBotID: String?
    /// The last quick ask's refusal, if the core refused it.
    public private(set) var error: String?
    /// Whether a quick ask is on its way to the core.
    public private(set) var isSending = false

    /// The conversation the last quick ask went into, and who it asked.
    private var asked: (store: ConversationStore, bot: Hearth_V1_Bot)?
    @ObservationIgnored private let conversation: @MainActor (Hearth_V1_Bot) -> ConversationStore

    /// A store over `bots` and `inbox`; `conversation` gives a bot's DM
    /// store, the same one the window shows.
    public init(
        bots: BotListStore, inbox: InboxStore,
        conversation: @escaping @MainActor (Hearth_V1_Bot) -> ConversationStore
    ) {
        self.bots = bots
        self.inbox = inbox
        self.conversation = conversation
    }

    /// Fetches the bots with their latest runs and the open items, for an
    /// extra opened while the window has not.
    public func load() async {
        await bots.load()
        await inbox.load()
    }

    /// Each bot whose latest run is queued or working, in the bots' order.
    public var working: [MenuBarPopover.Working] {
        bots.bots.compactMap { bot in
            guard let run = bots.latestRun(of: bot.id),
                run.state == .queued || run.state == .working
            else { return nil }
            return MenuBarPopover.Working(bot: bot, run: run)
        }
    }

    /// Each open item, newest first, with the bot it is about.
    public var waiting: [MenuBarPopover.Waiting] {
        inbox.items.map { item in
            MenuBarPopover.Waiting(item: item, bot: bots.bots.first { $0.id == item.botID })
        }
    }

    /// How many items are open: the sidebar's count.
    public var needsCount: Int {
        inbox.openCount
    }

    /// The status item's state.
    public var glyph: Glyph {
        Glyph(working: !working.isEmpty, needsYou: needsCount > 0)
    }

    /// The status item's spoken name: "Hearth, 2 working, 1 needs you".
    public var spokenName: String {
        var parts = ["Hearth"]
        if !working.isEmpty { parts.append("\(working.count) working") }
        if needsCount > 0 { parts.append("\(needsCount) needs you") }
        return parts.joined(separator: ", ")
    }

    /// Who the quick ask goes to: the bot the owner chose, else the Chief of
    /// Staff, first in the Office, else the first bot.
    public var bot: Hearth_V1_Bot? {
        bots.bots.first { $0.id == chosenBotID } ?? bots.office.first ?? bots.bots.first
    }

    /// The last quick ask's conversation, newest two messages of text, the
    /// owner's and the reply as it streams.
    public var reply: [Hearth_V1_Message] {
        guard let asked else { return [] }
        return Array(asked.store.messages.filter { $0.kind == .text }.suffix(2))
    }

    /// The bot the reply is from.
    public var replyBot: Hearth_V1_Bot? {
        asked?.bot
    }

    /// The conversation the reply is in, for opening it in the window.
    public var replyConversationID: String? {
        asked?.store.conversationID
    }

    /// Whether Send is enabled: someone to ask, words to send, nothing on
    /// its way, and that bot not busy with a run, as its latest run or the
    /// last quick ask's says; the window holds Send the same way.
    public var canAsk: Bool {
        guard let bot, !isSending,
            !draft.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        else { return false }
        let busy: [Hearth_V1_RunState] = [.queued, .working, .waitingOwner]
        if let state = bots.latestRun(of: bot.id)?.state, busy.contains(state) { return false }
        guard let asked, asked.bot.id == bot.id else { return true }
        return !asked.store.isWorking && !asked.store.isWaiting
    }

    /// Chooses who the quick ask goes to.
    public func pick(_ bot: Hearth_V1_Bot) {
        chosenBotID = bot.id
    }

    /// Sends the draft into the chosen bot's DM; false when there was nothing
    /// to send or the core refused it, and then the draft stays.
    @discardableResult
    public func ask() async -> Bool {
        guard canAsk, let bot else { return false }
        let sent = draft
        let store = conversation(bot)
        isSending = true
        defer { isSending = false }
        guard await store.send(sent.trimmingCharacters(in: .whitespacesAndNewlines)) else {
            error = store.error
            return false
        }
        asked = (store, bot)
        error = nil
        // Typing during the call keeps what was typed.
        if draft == sent { draft = "" }
        return true
    }
}
