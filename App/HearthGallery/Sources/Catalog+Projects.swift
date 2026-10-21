// Projects: the pie, a project's sidebar row in each light, done and selected,
// the whole sidebar with its sections, the header pinned over a project's
// conversation in each light, all done, done and overdue, the proposal card,
// and the project's conversation with a coordinator and without one.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var projects: [GalleryPage] {
        [
            GalleryPage(
                package: "HearthDesign", name: "ProgressPie",
                states: GalleryState.each(ProgressPie.PreviewState.self) {
                    ProgressPie(previewState: $0)
                }),
            GalleryPage(
                package: "HearthUI", name: "ProjectRow",
                states: GalleryState.each(
                    ProjectRow.PreviewState.self, width: Theme.Size.sidebarIdeal
                ) {
                    SidebarRowDemo(project: $0.project, isSelected: false)
                }
                    + [
                        GalleryState(
                            name: "selected", width: Theme.Size.sidebarIdeal,
                            view: AnyView(
                                SidebarRowDemo(
                                    project: ProjectRow.PreviewState.yellow.project,
                                    isSelected: true)))
                    ]),
            GalleryPage(
                package: "HearthFeatures", name: "SidebarList",
                states: botListStates + [sidebarWithoutProjects()]),
            GalleryPage(
                package: "HearthUI", name: "ProjectHeader",
                states: GalleryState.each(ProjectHeader.PreviewState.self, width: 560) {
                    ProjectHeader(previewState: $0).padding(Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthUI", name: "ProposalCard",
                states: GalleryState.each(ProposalCard.PreviewState.self, width: 560) {
                    ProposalCard(previewState: $0).padding(Theme.Spacing.lg)
                }),
        ]
    }

    /// The project's conversation: empty and answered, with rows, with nobody
    /// to answer, and the Chief of Staff's own conversation with the proposal.
    static var projectConversationStates: [GalleryState] {
        [
            projectConversation(name: "project, empty", conversationID: "empty"),
            projectConversation(
                name: "project, with rows", conversationID: Fixtures.dmConversation.id),
            projectConversation(
                name: "project, no coordinator", conversationID: "empty", coordinator: nil),
            proposalConversation(),
        ]
    }

    private static func projectConversation(
        name: String, conversationID: String, coordinator: Hearth_V1_Bot? = Fixtures.chiefOfStaff
    ) -> GalleryState {
        let client = PreviewClient()
        let store = ConversationStore(
            client: client, relay: EventRelay(client: client), conversationID: conversationID)
        return GalleryState(
            name: name, width: 560,
            view: AnyView(
                ConversationScreen(
                    store: store, project: Fixtures.projects[0], coordinator: coordinator
                )
                .frame(height: 560)))
    }

    /// The Chief of Staff's conversation with the card for the proposal made
    /// in it above the status line.
    private static func proposalConversation() -> GalleryState {
        let client = PreviewClient()
        let store = ConversationStore(
            client: client, relay: EventRelay(client: client),
            conversationID: Fixtures.chiefOfStaff.dmConversationID)
        return GalleryState(
            name: "proposal card", width: 560,
            view: AnyView(
                ConversationScreen(
                    store: store, bot: Fixtures.chiefOfStaff, proposals: [Fixtures.projects[3]]
                )
                .frame(height: 620)))
    }

    /// Everyone hired and no project yet: the "Projects" header over one line
    /// and the four example cards.
    private static func sidebarWithoutProjects() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let sidebar = SidebarStore(
            bots: BotListStore(client: client, relay: relay),
            projects: ProjectsStore(client: client, relay: relay))
        Task {
            for project in Fixtures.projects { await sidebar.projects.archive(id: project.id) }
        }
        return GalleryState(
            name: "no projects", width: Theme.Size.sidebarIdeal,
            view: AnyView(SidebarList(store: sidebar, onExample: { _ in }).frame(height: 560)))
    }
}

/// One project row in a sidebar list, so it is drawn as it is in the window,
/// selected or not.
private struct SidebarRowDemo: View {
    let project: Hearth_V1_Project
    let isSelected: Bool

    var body: some View {
        List(selection: .constant(isSelected ? project.id : nil)) {
            ProjectRow(project: project).tag(project.id)
        }
        .listStyle(.sidebar)
        .scrollDisabled(true)
        .frame(height: 56)
    }
}
