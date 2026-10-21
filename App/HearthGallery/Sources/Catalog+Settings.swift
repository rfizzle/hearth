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
                states: [usagePane(), deliverablesPane()]),
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

    /// The Deliverables pane: the five shipped templates, one saved by the
    /// owner and one whose file does not load.
    static func deliverablesPane() -> GalleryState {
        let client = PreviewClient()
        client.keepTemplate(
            .with {
                $0.id = "office-week"
                $0.name = "Office week"
                $0.when = "How the office did this week, with the tasks and the spend."
                $0.path = Fixtures.workspacePath + "/Templates/office-week.deliverable.json"
                $0.json = Fixtures.kitchenSinkJSON
            })
        client.keepTemplate(
            .with {
                $0.id = "board-update"
                $0.name = "Board update"
                $0.when = "A quarterly note for the board."
                $0.path = Fixtures.workspacePath + "/Templates/board-update.deliverable.json"
                $0.problem = "section 2: the example widget bar is not in allow"
            })
        let store = TemplatesStore(client: client)
        return GalleryState(
            name: "deliverables", width: Theme.Size.sheetWidth,
            view: AnyView(
                TemplatesPane(store: store).padding(Theme.Spacing.xl)))
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
