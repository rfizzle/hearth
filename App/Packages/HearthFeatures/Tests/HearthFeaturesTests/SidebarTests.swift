// The sidebar: the Inbox row's count follows the open items, and selecting
// the Inbox or a bot is one selection.

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
