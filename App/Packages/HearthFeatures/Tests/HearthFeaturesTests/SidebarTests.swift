// The sidebar: the Inbox row's count follows the open items, selecting the
// Inbox, a bot or a project is one selection, the Office holds the Chief of
// Staff, and a conversation selects what holds it.

import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
@Test func theInboxRowCountFollowsTheOpenItems() async {
    let client = PreviewClient()
    let inbox = InboxStore(client: client, relay: EventRelay(client: client))
    #expect(InboxRow.count(inbox.openCount) == nil)
    #expect(InboxRow.spoken(inbox.openCount) == "Nothing waiting")
    await inbox.load()
    #expect(InboxRow.count(inbox.openCount) == "4")
    #expect(InboxRow.spoken(inbox.openCount) == "4 items waiting")

    for item in [Fixtures.approval, Fixtures.widgetQuestion, Fixtures.question] {
        var closed = item
        closed.state = .closed
        await inbox.handle(.with { $0.event = .inboxChanged(.with { $0.item = closed }) })
    }
    #expect(InboxRow.count(inbox.openCount) == "1")
    #expect(InboxRow.spoken(inbox.openCount) == "1 item waiting")
}

/// A sidebar over the preview client's bots and projects.
@MainActor
func makeSidebar(client: any HearthClient = PreviewClient()) -> SidebarStore {
    let relay = EventRelay(client: client)
    return SidebarStore(
        bots: BotListStore(client: client, relay: relay),
        projects: ProjectsStore(client: client, relay: relay))
}

@MainActor
@Test func theInboxABotAndAProjectShareOneSelection() async {
    let sidebar = makeSidebar()
    await sidebar.bots.load()
    await sidebar.projects.load()
    sidebar.selected = .bot(Fixtures.bot.id)
    #expect(sidebar.selected == .bot(Fixtures.bot.id))
    #expect(sidebar.selectedBot?.id == Fixtures.bot.id)
    #expect(sidebar.selectedProject == nil)

    sidebar.selected = .inbox
    #expect(sidebar.selected == .inbox)
    #expect(sidebar.selectedBot == nil)
    #expect(sidebar.selectedProject == nil)

    sidebar.selected = .project(Fixtures.projects[0].id)
    #expect(sidebar.selected == .project(Fixtures.projects[0].id))
    #expect(sidebar.selectedProject?.id == Fixtures.projects[0].id)
    #expect(sidebar.selectedBot == nil)

    sidebar.selected = nil
    #expect(sidebar.selected == nil)
    #expect(sidebar.selectedBot == nil)
    #expect(sidebar.selectedProject == nil)
}

@MainActor
@Test func aBotsDotFollowsItsConversationsLatestRun() async {
    let idle = PreviewClient()
    let quiet = BotListStore(client: idle, relay: EventRelay(client: idle))
    await quiet.load()
    let dana = Fixtures.bot
    #expect(quiet.activity(of: dana) == nil)

    let busy = PreviewClient(latestRun: Fixtures.workingRun)
    let bots = BotListStore(client: busy, relay: EventRelay(client: busy))
    await bots.load()
    #expect(bots.activity(of: dana) == .working)

    var waiting = Fixtures.workingRun
    waiting.state = .waitingOwner
    await bots.handle(.with { $0.event = .runChanged(.with { $0.run = waiting }) })
    #expect(bots.activity(of: dana) == .waiting)
    #expect(BotListStore.Activity.waiting.spoken == "waiting for you")
    #expect(BotListStore.Activity.working.spoken == nil)

    await bots.handle(.with { $0.event = .runChanged(.with { $0.run = Fixtures.failedRun }) })
    #expect(bots.activity(of: dana) == nil)

    var paused = dana
    paused.status = .paused
    #expect(bots.activity(of: paused) == .paused)
}

@MainActor
@Test func theOfficeHoldsTheChiefOfStaffAboveTheTeam() async {
    let client = PreviewClient()
    let bots = BotListStore(client: client, relay: EventRelay(client: client))
    await bots.load()
    #expect(bots.office.map(\.id) == [Fixtures.chiefOfStaff.id])
    #expect(bots.team.map(\.id) == [Fixtures.bot.id])
    #expect(!bots.showsHireAffordance)
}

@MainActor
@Test func theChiefOfStaffIsSelectedOnlyWhenNothingIs() async {
    let sidebar = makeSidebar()
    sidebar.selectOfficeIfNothingSelected()
    #expect(sidebar.selected == nil)
    await sidebar.bots.load()
    sidebar.selectOfficeIfNothingSelected()
    #expect(sidebar.selected == .bot(Fixtures.chiefOfStaff.id))

    sidebar.selected = .bot(Fixtures.bot.id)
    sidebar.selectOfficeIfNothingSelected()
    #expect(sidebar.selected == .bot(Fixtures.bot.id))
    sidebar.selected = .inbox
    sidebar.selectOfficeIfNothingSelected()
    #expect(sidebar.selected == .inbox)
    sidebar.selected = .project(Fixtures.projects[0].id)
    sidebar.selectOfficeIfNothingSelected()
    #expect(sidebar.selected == .project(Fixtures.projects[0].id))
}

@MainActor
@Test func aConversationSelectsItsBotItsProjectOrTheInbox() async {
    let sidebar = makeSidebar()
    await sidebar.bots.load()
    await sidebar.projects.load()
    sidebar.show(conversationID: Fixtures.bot.dmConversationID)
    #expect(sidebar.selected == .bot(Fixtures.bot.id))
    let project = sidebar.projects.projects[0]
    sidebar.show(conversationID: project.conversationID)
    #expect(sidebar.selected == .project(project.id))
    sidebar.show(conversationID: "conversation-nobody-knows")
    #expect(sidebar.selected == .inbox)
}
