// BotListStore holds the bots as the core last reported them and which one is
// selected. It fetches the list, applies the bot changes the relay delivers,
// refetches on a resync, and decides nothing; the selection is what the sidebar highlights and the
// conversation opens.

import HearthKit
import Observation

/// The store behind the sidebar's bot list.
@MainActor
@Observable
public final class BotListStore {
    /// Every bot, in the order the core sent them; new ones come last.
    public private(set) var bots: [Hearth_V1_Bot] = []
    /// The selected bot's id, if one is selected.
    public var selection: String?
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    private var loaded = false
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

    /// Whether to offer hiring: the list has loaded and holds nobody.
    public var showsHireAffordance: Bool {
        loaded && bots.isEmpty
    }

    /// The selected bot, if it is still in the list.
    public var selectedBot: Hearth_V1_Bot? {
        bots.first { $0.id == selection }
    }

    /// Fetches the list from the core.
    public func load() async {
        do {
            bots = try await client.listBots()
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .botChanged(let change): upsert(change.bot)
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
