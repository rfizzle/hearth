// The projects in the sidebar: the list loads without the proposal, a new
// project is the last active one and selectable, done moves a row after the
// active ones, archive drops it, an event from the core replaces a row, a
// resync refetches, and the sheet's one check is that every field is typed.

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
    var form = ProjectForm()
    form.name = "Plan the summer party"
    form.goal = "A party for the studio and its clients."
    form.context = "Last year's was in the courtyard."
    form.doneLooksLike = "A date, a venue and the invitations sent."
    form.doneWhen = "The invitations have gone out."
    let created = try #require(await sidebar.projects.create(form.createRequest))
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
    var form = ProjectForm()
    form.name = String(repeating: "a", count: 81)
    #expect(await store.create(form.createRequest) == nil)
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

@Test func theSheetSavesOnceEveryFieldIsTyped() {
    var form = ProjectForm()
    #expect(!form.canSave)
    form.name = "Launch the newsletter"
    form.goal = "Send the first issue."
    form.context = "400 subscribers."
    form.doneLooksLike = "A template and a first issue."
    #expect(!form.canSave)
    form.doneWhen = "  "
    #expect(!form.canSave)
    form.doneWhen = "The first issue is out."
    #expect(form.canSave)
    #expect(form.createRequest.targetDate.isEmpty)
}

@Test func anEditedProjectKeepsItsBriefAndDate() throws {
    let project = Fixtures.projects[0]
    let form = ProjectForm(project: project)
    #expect(form.canSave)
    #expect(form.hasTargetDate)
    let request = form.updateRequest(id: project.id)
    #expect(request.id == project.id)
    #expect(request.name == project.name)
    #expect(request.brief == project.brief)
    #expect(request.targetDate == "2026-11-20")
    #expect(!ProjectForm(project: Fixtures.projects[1]).hasTargetDate)
}
