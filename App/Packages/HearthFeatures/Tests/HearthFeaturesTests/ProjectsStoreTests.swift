// The projects in the sidebar: the list loads without the proposal, a new
// project is the last active one and selectable, done moves a row after the
// active ones, archive drops it, an event from the core replaces a row, a
// resync refetches, and a refused create keeps the core's words.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
@Test func theProjectsListWithoutTheProposalActiveFirst() async {
    let sidebar = makeSidebar()
    await sidebar.projects.load()
    #expect(
        sidebar.projects.listed.map(\.id) == [
            "project-newsletter", "project-office", "project-tax",
        ])
    #expect(sidebar.projects.proposals.map(\.id) == ["project-website"])
    #expect(sidebar.projects.error == nil)
}

@MainActor
@Test func aCreatedProjectFollowsTheActiveOnesAndIsSelected() async throws {
    let sidebar = makeSidebar()
    await sidebar.projects.load()
    let request = Hearth_V1_CreateProjectRequest.with {
        $0.name = "Plan the summer party"
        $0.brief = .with {
            $0.goal = "A party for the studio and its clients."
            $0.context = "Last year's was in the courtyard."
            $0.doneLooksLike = "A date, a venue and the invitations sent."
            $0.doneWhen = "The invitations have gone out."
        }
    }
    let created = try #require(await sidebar.projects.create(request))
    #expect(
        sidebar.projects.listed.map(\.id) == [
            "project-newsletter", "project-office", created.id, "project-tax",
        ])
    #expect(sidebar.projects.saveError == nil)

    sidebar.selected = .project(created.id)
    #expect(sidebar.selectedProject?.name == "Plan the summer party")
}

@MainActor
@Test func aRefusedCreateKeepsTheCoresWords() async {
    let store = makeSidebar().projects
    await store.load()
    let request = Hearth_V1_CreateProjectRequest.with {
        $0.name = String(repeating: "a", count: 81)
    }
    #expect(await store.create(request) == nil)
    #expect(store.saveError == "invalid project: the name is over 80 characters")
    #expect(store.listed.count == 3)
    store.clearSaveError()
    #expect(store.saveError == nil)
}

@MainActor
@Test func markingDoneMovesTheRowAfterTheActiveOnes() async {
    let store = makeSidebar().projects
    await store.load()
    await store.markDone(id: "project-newsletter")
    #expect(store.project(id: "project-newsletter")?.state == .done)
    #expect(store.listed.map(\.id) == ["project-office", "project-newsletter", "project-tax"])
}

@MainActor
@Test func archivingDropsTheRowAndItsSelection() async {
    let sidebar = makeSidebar()
    await sidebar.projects.load()
    sidebar.selected = .project("project-tax")
    await sidebar.projects.archive(id: "project-tax")
    #expect(sidebar.projects.listed.map(\.id) == ["project-newsletter", "project-office"])
    #expect(sidebar.selectedProject == nil)
}

@MainActor
@Test func aProjectChangeFromTheCoreReplacesItsRow() async {
    let store = makeSidebar().projects
    await store.load()
    for event in Fixtures.projectEvents { await store.handle(event) }
    #expect(store.project(id: "project-newsletter")?.state == .done)

    var paused = Fixtures.projects[1]
    paused.status.tasksTotal = 2
    paused.status.tasksPausedBudget = 1
    paused.status.light = .red
    paused.status.lightLabel = "Paused on budget"
    await store.handle(.with { $0.event = .projectChanged(.with { $0.project = paused }) })
    #expect(store.project(id: "project-office")?.status.lightLabel == "Paused on budget")
    #expect(store.listed.count == 3)

    var archived = paused
    archived.state = .archived
    await store.handle(.with { $0.event = .projectChanged(.with { $0.project = archived }) })
    #expect(store.project(id: "project-office") == nil)

    await store.handle(.with { $0.event = .resync(.init()) })
    #expect(store.listed.count == 3)
}

@MainActor
@Test func confirmingAProposalRemovesTheCardAndSelectsTheProject() async {
    let sidebar = makeSidebar()
    await sidebar.projects.load()
    let proposal = Fixtures.projects[3]
    #expect(sidebar.projects.proposals(in: proposal.proposedInConversationID).count == 1)

    await sidebar.confirm(proposal)
    #expect(sidebar.projects.proposals.isEmpty)
    #expect(sidebar.selected == .project(proposal.id))
    #expect(sidebar.selectedProject?.state == .active)
    #expect(sidebar.selectedProject?.name == "Refresh the website")
    #expect(sidebar.projects.error == nil)
}

@MainActor
@Test func dismissingAProposalRemovesTheCardAndListsNothing() async {
    let sidebar = makeSidebar()
    await sidebar.projects.load()
    await sidebar.projects.dismiss(Fixtures.projects[3])
    #expect(sidebar.projects.proposals.isEmpty)
    #expect(sidebar.projects.project(id: "project-website") == nil)
    #expect(sidebar.selected == nil)
}

@MainActor
@Test func aProposalBelongsToTheConversationItWasMadeIn() async {
    let projects = makeSidebar().projects
    await projects.load()
    #expect(projects.proposals(in: Fixtures.chiefOfStaff.dmConversationID).count == 1)
    #expect(projects.proposals(in: Fixtures.dmConversation.id).isEmpty)
}
