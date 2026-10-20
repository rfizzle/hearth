// ModelsStore holds which model each tier asks for and every model the owner
// may pick, as the core last reported them. It fetches them, applies the tier
// change the relay delivers, refetches on a resync, and turns a pick into a
// call whose answer is the mapping the core kept. It decides nothing: the core
// says what is offered and what a tier resolves to. The Settings pane and the
// Access sheet's Advanced choice share the one store.
// See docs/model.md#providers-and-models

import Foundation
import HearthKit
import HearthUI
import Observation

/// The store behind the Models pane and the Advanced model choice.
@MainActor
@Observable
public final class ModelsStore {
    /// The model each tier asks for.
    public private(set) var tiers: [Hearth_V1_TierChoice] = []
    /// Every model the owner may pick, grouped by their labels in the views.
    public private(set) var options: [Hearth_V1_ModelOption] = []
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

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

    /// Fetches the tiers and the options, replacing what is shown.
    public func load() async {
        do {
            let response = try await client.tiers()
            tiers = response.tiers
            options = response.options
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// The tiers the pane shows, in the order the owner reads them.
    public static let order: [Hearth_V1_Tier] = [.fast, .balanced, .best]

    /// The model `tier` asks for: the option the choice names, or one made
    /// from the choice when the accounts no longer offer it; nil when the
    /// core has not said.
    public func selection(for tier: Hearth_V1_Tier) -> Hearth_V1_ModelOption? {
        guard let choice = tiers.first(where: { $0.tier == tier }) else { return nil }
        return options.first { $0.matches(choice) }
            ?? .with {
                $0.provider = choice.provider
                $0.profile = choice.profile
                $0.model = choice.model
            }
    }

    /// Sends `option` as the model for `tier`, keeping the other tiers, and
    /// shows the mapping the core kept.
    public func set(tier: Hearth_V1_Tier, option: Hearth_V1_ModelOption) async {
        var next = tiers.filter { $0.tier != tier }
        next.append(
            .with {
                $0.tier = tier
                $0.provider = option.provider
                $0.profile = option.profile
                $0.model = option.model
            })
        do {
            tiers = try await client.setTiers(next)
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .tiersChanged(let change): tiers = change.tiers
        case .resync: await load()
        default: break
        }
    }
}
