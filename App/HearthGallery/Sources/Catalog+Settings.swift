// What the Settings window shows beyond the sign-in panels: the Usage pane
// over the fixture's two months, the Models pane with the three tiers, and a
// saved service's card in its edit state.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var settings: [GalleryPage] {
        [
            GalleryPage(
                package: "HearthFeatures", name: "Settings",
                states: [usagePane()]),
            GalleryPage(
                package: "HearthFeatures", name: "ModelsPane", states: [modelsPane()]),
            GalleryPage(
                package: "HearthUI", name: "ProviderCard",
                states: GalleryState.each(ProviderCard.PreviewState.self, width: 480) {
                    ProviderCard(previewState: $0)
                }),
        ]
    }

    /// The Models pane over the fixture's tiers: Fast and Balanced on ChatGPT,
    /// Best on Anthropic, with a local model among the options.
    static func modelsPane() -> GalleryState {
        let client = PreviewClient()
        return GalleryState(
            name: "tiers", width: Theme.Size.sheetWidth,
            view: AnyView(
                ModelsPane(store: ModelsStore(client: client, relay: EventRelay(client: client)))
                    .padding(Theme.Spacing.xl)))
    }

    /// The Usage pane for Dana, near her budget, with the team above her.
    static func usagePane() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let bots = BotListStore(client: client, relay: relay)
        return GalleryState(
            name: "usage", width: Theme.Size.sheetWidth,
            view: AnyView(
                UsagePane(usage: UsageStore(client: client, relay: relay), bots: bots)
                    .task { await bots.load() }
                    .padding(Theme.Spacing.xl)))
    }
}
