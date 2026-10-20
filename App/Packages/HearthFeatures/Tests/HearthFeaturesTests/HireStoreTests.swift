// Store tests for hiring and the bot list, against PreviewClient and a
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
@Test func pickingATemplatePrefillsTheFieldsButNotTheName() async {
    let store = HireStore(client: PreviewClient())
    await store.load()
    #expect(store.templates.count == Fixtures.templates.count)
    store.name = "Sam"
    let writer = Fixtures.templates[1]
    store.pick(writer)
    #expect(store.start == .template(writer.id))
    #expect(store.role == writer.role)
    #expect(store.jobDescription == writer.jobDescription)
    #expect(store.tier == .best)
    #expect(store.name == "Sam")
}

@MainActor
@Test func describingTheJobStartsBlankAtBalanced() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[1])
    store.describe()
    #expect(store.start == .describe)
    #expect(store.role.isEmpty)
    #expect(store.jobDescription.isEmpty)
    #expect(store.tier == .balanced)
}

@MainActor
@Test func createIsOffWhileTheNameOrDescriptionIsBlank() async {
    let store = HireStore(client: PreviewClient())
    #expect(!store.canCreate)
    store.name = "  "
    store.jobDescription = "Water the plants."
    #expect(!store.canCreate)
    store.name = "Sam"
    store.jobDescription = " "
    #expect(!store.canCreate)
    store.jobDescription = "Water the plants."
    #expect(store.canCreate)
    store.name = ""
    await store.create()
    #expect(store.createdBot == nil)
}

@MainActor
@Test func createSendsWhatWasTyped() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[0])
    store.name = "Sam"
    store.role = "Plant Lead"
    store.jobDescription = "You water the plants."
    store.tier = .fast
    store.avatar = "🌱"
    await store.create()
    #expect(store.createdBot?.name == "Sam")
    #expect(store.createdBot?.role == "Plant Lead")
    #expect(store.createdBot?.jobDescription == "You water the plants.")
    #expect(store.createdBot?.tier == .fast)
    #expect(store.createdBot?.avatar == "🌱")
    #expect(store.error == nil)
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

@MainActor
@Test func improveShowsAProposalAndLeavesTheOwnersTextAlone() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[0])
    store.name = "Sam"
    await store.improve()
    #expect(store.proposal?.name == Fixtures.draft.name)
    #expect(store.proposal?.jobDescription == Fixtures.draft.jobDescription)
    #expect(store.jobDescription == Fixtures.templates[0].jobDescription)
    #expect(store.name == "Sam")
    #expect(store.error == nil)
}

@MainActor
@Test func improveIsOffWithNothingToImprove() async {
    let store = HireStore(client: PreviewClient())
    #expect(!store.canDraft)
    await store.improve()
    #expect(store.proposal == nil)
}

@MainActor
@Test func usingTheProposalMovesItIntoTheFormAndCreateSendsIt() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[0])
    await store.improve()
    store.proposal?.workingStyle = "Warm and patient"
    store.useProposal()
    #expect(store.proposal == nil)
    #expect(store.name == Fixtures.draft.name)
    #expect(store.role == Fixtures.draft.role)
    #expect(store.tier == Fixtures.draft.tier)
    let shown = Fixtures.draft.jobDescription + "\n\nWorking style: Warm and patient"
    #expect(store.jobDescription == shown)
    await store.create()
    #expect(store.createdBot?.jobDescription == shown)
    #expect(store.createdBot?.name == Fixtures.draft.name)
}

@MainActor
@Test func keepingMineDropsTheProposalAndChangesNothing() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[1])
    store.name = "Sam"
    await store.improve()
    store.keepMine()
    #expect(store.proposal == nil)
    #expect(store.name == "Sam")
    #expect(store.role == Fixtures.templates[1].role)
    #expect(store.jobDescription == Fixtures.templates[1].jobDescription)
    #expect(store.tier == .best)
}

@MainActor
@Test func aChipAddsOneVisibleSentenceAndTheNextChipReplacesIt() async {
    let store = HireStore(client: PreviewClient())
    store.describe()
    store.jobDescription = "Keep my plants alive."
    await store.improve()
    let plain = Fixtures.draft.jobDescription
    store.choose(.warm)
    #expect(store.proposal?.jobDescription == plain + " " + HireStore.Style.warm.sentence)
    #expect(store.proposal?.style == .warm)
    store.choose(.thorough)
    #expect(store.proposal?.jobDescription == plain + " " + HireStore.Style.thorough.sentence)
    store.choose(.thorough)
    #expect(store.proposal?.jobDescription == plain)
    #expect(store.proposal?.style == nil)
    #expect(store.jobDescription == "Keep my plants alive.")
}

@MainActor
@Test func pickingAnotherStartDropsTheProposal() async {
    let store = HireStore(client: PreviewClient())
    store.pick(Fixtures.templates[0])
    await store.improve()
    store.describe()
    #expect(store.proposal == nil)
}
