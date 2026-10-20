// The pieces of the hire sheet, and the team in the sidebar.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var hire: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "TemplateCard",
                states: GalleryState.each(TemplateCard.PreviewState.self, width: 240) {
                    TemplateCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "TierPicker",
                states: GalleryState.each(TierPicker.PreviewState.self, width: 480) {
                    TierPicker(previewState: $0)
                }),
        ]
    }

    /// The sidebar with a team whose bots are idle, working and paused, and
    /// the empty team: the header alone beside the detail column's invitation.
    @MainActor
    static var botListStates: [GalleryState] {
        var working = Fixtures.bot
        working.id = "bot-sam"
        working.name = "Sam"
        working.role = "Writer"
        working.status = .working
        working.avatar = ""
        var paused = Fixtures.bot
        paused.id = "bot-lee"
        paused.name = "Lee"
        paused.role = "Scheduler"
        paused.status = .paused
        paused.avatar = "🗓"
        let changes = [working, paused].map { bot in
            Hearth_V1_SubscribeResponse.with { $0.event = .botChanged(.with { $0.bot = bot }) }
        }
        let events = AsyncStream<Hearth_V1_SubscribeResponse> { continuation in
            changes.forEach { continuation.yield($0) }
            continuation.finish()
        }
        let team = BotListStore(client: PreviewClient(), events: events)
        team.selection = Fixtures.bot.id
        let empty = BotListStore(client: noBotsClient, events: AsyncStream { $0.finish() })
        return [
            GalleryState(
                name: "idle, working, paused", width: 280,
                view: AnyView(BotList(store: team, onHire: {}).frame(height: 240))),
            GalleryState(
                name: "none", width: 640,
                view: AnyView(
                    HStack(spacing: 0) {
                        BotList(store: empty, onHire: {}).frame(width: Theme.Size.sidebarIdeal)
                        Divider()
                        TeamEmptyView(onHire: {})
                    }
                    .frame(height: 280))),
        ]
    }
}
