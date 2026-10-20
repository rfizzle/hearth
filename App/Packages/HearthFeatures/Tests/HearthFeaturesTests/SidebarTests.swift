// The sidebar: the Inbox row's count follows the open items, selecting the
// Inbox or a bot is one selection, and the Office holds the Chief of Staff.

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

@MainActor
@Test func theInboxAndABotShareOneSelection() async {
    let client = PreviewClient()
    let bots = BotListStore(client: client, relay: EventRelay(client: client))
    await bots.load()
    bots.selection = Fixtures.bot.id
    #expect(bots.selected == .bot(Fixtures.bot.id))
    #expect(bots.selectedBot?.id == Fixtures.bot.id)

    bots.selected = .inbox
    #expect(bots.selection == nil)
    #expect(bots.selectedBot == nil)

    bots.selection = nil
    #expect(bots.selected == nil)
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
    let client = PreviewClient()
    let bots = BotListStore(client: client, relay: EventRelay(client: client))
    bots.selectOfficeIfNothingSelected()
    #expect(bots.selected == nil)
    await bots.load()
    bots.selectOfficeIfNothingSelected()
    #expect(bots.selection == Fixtures.chiefOfStaff.id)

    bots.selection = Fixtures.bot.id
    bots.selectOfficeIfNothingSelected()
    #expect(bots.selection == Fixtures.bot.id)
    bots.selected = .inbox
    bots.selectOfficeIfNothingSelected()
    #expect(bots.selected == .inbox)
}
