// What the Settings window shows beyond the sign-in panels: the Usage pane
// over the fixture's two months. The next pane joins this list.

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
                states: [usagePane()])
        ]
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
