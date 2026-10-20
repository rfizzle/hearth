// BotListStore holds the bots as the core last reported them. It fetches the
// list, applies the bot changes the relay delivers, refetches on a resync, and
// decides nothing; what the sidebar has selected is SidebarStore's.
// Each bot's row dot follows its conversation's latest run, read once per bot
// from the core and moved by the run changes after: working, or waiting for
// the owner in the attention tone. The org-level bots, the Chief of Staff
// first, are the sidebar's Office and everyone else its team, by the scope
// the core reports.
// See docs/model.md#8-approvals and docs/model.md#people

import HearthKit
import Observation

/// The store behind the sidebar's bot list.
@MainActor
@Observable
public final class BotListStore {
    /// Every bot, in the order the core sent them; new ones come last.
    public private(set) var bots: [Hearth_V1_Bot] = []
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    /// What a bot's row dot says.
    public enum Activity: Equatable, Sendable {
        case working
        case waiting
        case paused

        /// What VoiceOver adds to the row, if anything.
        public var spoken: String? {
            self == .waiting ? "waiting for you" : nil
        }
    }

    /// Where paused comes from: the core has no paused status for a bot, only
    /// the usage summary's flag. Set by the window's session.
    @ObservationIgnored public weak var usage: UsageStore?

    private var loaded = false
    private var runs: [String: Hearth_V1_Run] = [:]
    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay) {
        self.client = client
        self.relay = relay
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// The org-level bots, in the core's order: the sidebar's Office.
    public var office: [Hearth_V1_Bot] {
        bots.filter { $0.scope == .org }
    }

    /// Every other bot, in the core's order: the sidebar's team.
    public var team: [Hearth_V1_Bot] {
        bots.filter { $0.scope != .org }
    }

    /// Whether to offer hiring: the list has loaded and nobody is on the team.
    public var showsHireAffordance: Bool {
        loaded && team.isEmpty
    }

    /// What `bot`'s row dot says: waiting for the owner, working, paused, or
    /// nothing. The latest run outranks the bot's own status.
    public func activity(of bot: Hearth_V1_Bot) -> Activity? {
        switch runs[bot.id]?.state {
        case .waitingOwner: return .waiting
        case .queued, .working: return .working
        default: break
        }
        if usage?.isPaused(bot.id) == true { return .paused }
        return switch bot.status {
        case .working: .working
        case .paused: .paused
        default: nil
        }
    }

    /// `botID`'s latest run as the core last reported it, if it has one; the
    /// menu bar's Working rows read it.
    public func latestRun(of botID: String) -> Hearth_V1_Run? {
        runs[botID]
    }

    /// Whether `botID` is paused this month; the sidebar's row shows it.
    public func isPaused(_ botID: String) -> Bool {
        usage?.isPaused(botID) ?? false
    }

    /// Fetches the list from the core, then each bot's latest run.
    public func load() async {
        do {
            bots = try await client.listBots()
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
            return
        }
        for bot in bots {
            // A conversation that cannot be read leaves its bot's dot to the
            // bot's own status.
            guard let list = try? await client.listConversations(botID: bot.id) else { continue }
            if let latest = list.filter(\.hasLatestRun).map(\.latestRun).last {
                runs[bot.id] = latest
            } else {
                runs[bot.id] = nil
            }
        }
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .botChanged(let change): upsert(change.bot)
        case .runChanged(let change): runs[change.run.botID] = change.run
        case .resync: await load()
        default: break
        }
    }

    private func upsert(_ bot: Hearth_V1_Bot) {
        if let index = bots.firstIndex(where: { $0.id == bot.id }) {
            bots[index] = bot
        } else {
            bots.append(bot)
        }
    }
}
