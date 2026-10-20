// Store tests for the bot list, against PreviewClient and a
// scripted stream of core events.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func botList(client: any HearthClient = PreviewClient()) -> BotListStore {
    BotListStore(client: client, relay: EventRelay(client: client))
}

private func botEvent(_ bot: Hearth_V1_Bot) -> Hearth_V1_SubscribeResponse {
    .with { $0.event = .botChanged(.with { $0.bot = bot }) }
}

@MainActor
@Test func aNewBotAppearsAfterABotChangedEvent() async {
    var sam = Fixtures.bot
    sam.id = "bot-sam"
    sam.name = "Sam"
    let store = botList()
    await store.load()
    #expect(store.team.map(\.name) == ["Dana"])
    await store.handle(botEvent(sam))
    #expect(store.team.map(\.name) == ["Dana", "Sam"])
}

@MainActor
@Test func aChangedBotReplacesItsRowAndKeepsTheSelection() async {
    var renamed = Fixtures.bot
    renamed.status = .working
    let store = botList()
    let client = PreviewClient()
    let sidebar = SidebarStore(
        bots: store, projects: ProjectsStore(client: client, relay: EventRelay(client: client)))
    await store.load()
    sidebar.selected = .bot(Fixtures.bot.id)
    await store.handle(botEvent(renamed))
    #expect(store.team.count == 1)
    #expect(sidebar.selectedBot?.status == .working)
}

@MainActor
@Test func anEmptyListShowsTheHireAffordanceOnceLoaded() async {
    let store = botList(client: EmptyOrgClient())
    #expect(!store.showsHireAffordance)
    await store.load()
    #expect(store.showsHireAffordance)
    let full = botList()
    await full.load()
    #expect(!full.showsHireAffordance)
}

@MainActor
@Test func aResyncRefetchesTheList() async {
    var stray = Fixtures.bot
    stray.id = "bot-stray"
    let store = botList()
    await store.load()
    await store.handle(botEvent(stray))
    #expect(store.team.count == 2)
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.bots.map(\.id) == [Fixtures.chiefOfStaff.id, Fixtures.bot.id])
}
