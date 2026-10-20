// The quality picker, and the team in the sidebar.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var team: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "TierPicker",
                states: GalleryState.each(TierPicker.PreviewState.self, width: 480) {
                    TierPicker(previewState: $0)
                })
        ]
    }

    /// The sidebar with a team whose bots are idle, working and paused above
    /// the projects, and the empty team with one project: the team's header
    /// alone beside the detail column's invitation.
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
        // The changes arrive after the list's own fetch, as they would live.
        let relay = EventRelay(subscribe: { _ in
            AsyncThrowingStream { continuation in
                let task = Task {
                    try? await Task.sleep(for: .milliseconds(300))
                    changes.forEach { continuation.yield($0) }
                }
                continuation.onTermination = { _ in task.cancel() }
            }
        })
        let client = PreviewClient()
        let team = SidebarStore(
            bots: BotListStore(client: client, relay: relay),
            projects: ProjectsStore(client: client, relay: relay),
            selected: .bot(Fixtures.bot.id))
        relay.start()
        let emptyClient = noBotsClient
        let emptyRelay = EventRelay(client: emptyClient)
        let empty = SidebarStore(
            bots: BotListStore(client: emptyClient, relay: emptyRelay),
            projects: ProjectsStore(client: emptyClient, relay: emptyRelay))
        Task {
            for id in ["project-office", "project-tax"] { await empty.projects.archive(id: id) }
        }
        return [
            GalleryState(
                name: "the office, the team idle, working, paused, then the projects", width: 280,
                view: AnyView(SidebarList(store: team).frame(height: 520))),
            GalleryState(
                name: "nobody hired, one project", width: 640,
                view: AnyView(
                    HStack(spacing: 0) {
                        SidebarList(store: empty).frame(width: Theme.Size.sidebarIdeal)
                        Divider()
                        TeamEmptyView(onHire: {})
                    }
                    .frame(height: 320))),
        ]
    }
}
