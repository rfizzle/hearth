// BotListStore holds the bots as the core last reported them and which one is
// selected. It fetches the list, applies the bot changes the core announces,
// and decides nothing; the selection is what the sidebar highlights and the
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
    private let events: AsyncStream<Hearth_V1_SubscribeResponse>

    /// A store over `client`; `events` is the relay's stream of core events.
    public init(client: any HearthClient, events: AsyncStream<Hearth_V1_SubscribeResponse>) {
        self.client = client
        self.events = events
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

    /// Applies bot changes until the stream ends.
    public func run() async {
        for await event in events {
            apply(event)
        }
    }

    private func apply(_ event: Hearth_V1_SubscribeResponse) {
        guard case .botChanged(let change) = event.event else { return }
        if let index = bots.firstIndex(where: { $0.id == change.bot.id }) {
            bots[index] = change.bot
        } else {
            bots.append(change.bot)
        }
    }
}
