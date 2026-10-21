// What a bot can see and do: the capability row, the folder row and the sheet
// that holds them.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var access: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "AccessRow",
                states: GalleryState.each(AccessRow.PreviewState.self, width: 560) {
                    AccessRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: package, name: "FolderRow",
                states: GalleryState.each(FolderRow.PreviewState.self, width: 560) {
                    FolderRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "AccessSheet", states: accessSheetStates),
        ]
    }

    /// The sheet at the core's defaults, and again once the core reports a
    /// second folder: the file rows then stand for both, at the same modes.
    @MainActor
    static var accessSheetStates: [GalleryState] {
        let client = PreviewClient()
        let extra = "/Users/preview/Documents/Pricing"
        let widened = Fixtures.grants.flatMap { grant -> [Hearth_V1_Grant] in
            guard !grant.folder.isEmpty else { return [grant] }
            var other = grant
            other.folder = extra
            other.ownFolder = false
            return [grant, other]
        }
        let change = Hearth_V1_SubscribeResponse.with {
            $0.sequence = 1
            $0.event = .grantsChanged(
                .with {
                    $0.botID = Fixtures.bot.id
                    $0.grants = widened
                })
        }
        // The change arrives after the sheet's own fetch, as it would live.
        let relay = EventRelay(subscribe: { _ in
            AsyncThrowingStream { continuation in
                let task = Task {
                    try? await Task.sleep(for: .milliseconds(300))
                    continuation.yield(change)
                }
                continuation.onTermination = { _ in task.cancel() }
            }
        })
        relay.start()
        let models = ModelsStore(client: client, relay: EventRelay(client: client))
        func advanced(_ override: String = "") -> BotModelStore {
            var bot = Fixtures.bot
            bot.modelOverride = override
            return BotModelStore(client: client, models: models, bot: bot)
        }
        let defaults = GrantsStore(
            client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id)
        let widenedStore = GrantsStore(client: client, relay: relay, botID: Fixtures.bot.id)
        return [
            GalleryState(
                name: "defaults", width: Theme.Size.sheetWidth,
                view: AnyView(
                    AccessSheet(
                        store: defaults, advanced: advanced(), botName: Fixtures.bot.name,
                        onDone: {}))),
            GalleryState(
                name: "extra folder", width: Theme.Size.sheetWidth,
                view: AnyView(
                    AccessSheet(
                        store: widenedStore, advanced: advanced(), botName: Fixtures.bot.name,
                        onDone: {}))),
            GalleryState(
                name: "advanced", width: Theme.Size.sheetWidth,
                view: AnyView(
                    AccessSheet(
                        store: GrantsStore(
                            client: client, relay: EventRelay(client: client),
                            botID: Fixtures.bot.id),
                        advanced: advanced("anthropic::claude-opus-4-5"),
                        botName: Fixtures.bot.name, advancedOpen: true, onDone: {}))),
        ]
    }
}
