// Store tests for hiring and the bot list, against PreviewClient and a
// scripted stream of core events.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

/// A client with nobody hired yet; everything else is PreviewClient's.
private struct EmptyOrgClient: HearthClient {
    let base = PreviewClient()

    func ping() async throws -> String { try await base.ping() }
    func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        base.streamHello(count: count)
    }
    func getOrg() async throws -> Hearth_V1_Org? { try await base.getOrg() }
    func updateOrg(name: String, workspacePath: String) async throws -> Hearth_V1_Org {
        try await base.updateOrg(name: name, workspacePath: workspacePath)
    }
    func listTemplates() async throws -> [Hearth_V1_RoleTemplate] { try await base.listTemplates() }
    func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot {
        try await base.createBot(request)
    }
    func draftBot(
        _ request: Hearth_V1_DraftBotRequest
    ) async throws -> Hearth_V1_DraftBotResponse {
        try await base.draftBot(request)
    }
    func listBots() async throws -> [Hearth_V1_Bot] { [] }
    func listConversations(botID: String) async throws -> [Hearth_V1_Conversation] {
        try await base.listConversations(botID: botID)
    }
    func listMessages(
        conversationID: String, beforeSeq: Int64, limit: Int32
    ) async throws -> Hearth_V1_ListMessagesResponse {
        try await base.listMessages(
            conversationID: conversationID, beforeSeq: beforeSeq, limit: limit)
    }
    func send(
        conversationID: String, text: String, attachmentPaths: [String]
    ) async throws -> Hearth_V1_SendResponse {
        try await base.send(
            conversationID: conversationID, text: text, attachmentPaths: attachmentPaths)
    }
    func cancel(runID: String) async throws { try await base.cancel(runID: runID) }
    func getConversation(id: String) async throws -> Hearth_V1_Conversation {
        try await base.getConversation(id: id)
    }
    func retry(runID: String) async throws -> Hearth_V1_Run { try await base.retry(runID: runID) }
    func listGrants(botID: String) async throws -> [Hearth_V1_Grant] {
        try await base.listGrants(botID: botID)
    }
    func setGrants(botID: String, _ grants: [Hearth_V1_Grant]) async throws -> [Hearth_V1_Grant] {
        try await base.setGrants(botID: botID, grants)
    }
    func listInbox(includeClosed: Bool) async throws -> [Hearth_V1_InboxItem] {
        try await base.listInbox(includeClosed: includeClosed)
    }
    func decide(
        itemID: String, _ decision: Hearth_V1_Decision, reason: String
    ) async throws -> Hearth_V1_DecideResponse {
        try await base.decide(itemID: itemID, decision, reason: reason)
    }
    func answer(
        itemID: String, _ text: String, value: Hearth_V1_WidgetAnswer?
    ) async throws -> Hearth_V1_AnswerResponse {
        try await base.answer(itemID: itemID, text, value: value)
    }
    func acknowledge(itemID: String) async throws -> Hearth_V1_InboxItem {
        try await base.acknowledge(itemID: itemID)
    }
    func listAccounts() async throws -> [Hearth_V1_ProviderAccount] {
        try await base.listAccounts()
    }
    func setAPIKey(
        provider: Hearth_V1_Provider, key: String
    ) async throws -> Hearth_V1_ProviderAccount {
        try await base.setAPIKey(provider: provider, key: key)
    }
    func clearCredential(provider: Hearth_V1_Provider) async throws {
        try await base.clearCredential(provider: provider)
    }
    func saveService(
        _ request: Hearth_V1_SetAPIKeyRequest
    ) async throws -> Hearth_V1_ProviderAccount {
        try await base.saveService(request)
    }
    func removeService(profile: String) async throws {
        try await base.removeService(profile: profile)
    }
    func listKnownServices() async throws -> [Hearth_V1_KnownService] {
        try await base.listKnownServices()
    }
    func startChatGPTLogin() async throws -> URL { try await base.startChatGPTLogin() }
    func listIntegrations() async throws -> [Hearth_V1_IntegrationAccount] {
        try await base.listIntegrations()
    }
    func setIntegrationKey(
        _ integration: Hearth_V1_Integration, key: String
    ) async throws -> Hearth_V1_IntegrationAccount {
        try await base.setIntegrationKey(integration, key: key)
    }
    func clearIntegration(_ integration: Hearth_V1_Integration) async throws {
        try await base.clearIntegration(integration)
    }
    func updateBot(_ request: Hearth_V1_UpdateBotRequest) async throws -> Hearth_V1_Bot {
        try await base.updateBot(request)
    }
    func listNotes(botID: String) async throws -> Hearth_V1_ListNotesResponse {
        try await base.listNotes(botID: botID)
    }
    func editNote(id: String, text: String) async throws -> Hearth_V1_Note {
        try await base.editNote(id: id, text: text)
    }
    func deleteNote(id: String) async throws { try await base.deleteNote(id: id) }
    func pinNote(id: String, pinned: Bool) async throws -> Hearth_V1_Note {
        try await base.pinNote(id: id, pinned: pinned)
    }
    func listPages(botID: String) async throws -> Hearth_V1_ListPagesResponse {
        try await base.listPages(botID: botID)
    }
    func putPage(_ request: Hearth_V1_PutPageRequest) async throws -> Hearth_V1_PutPageResponse {
        try await base.putPage(request)
    }
    func deletePage(id: String) async throws -> Hearth_V1_ContextMeter {
        try await base.deletePage(id: id)
    }
    func usageSummary(month: String) async throws -> Hearth_V1_SummaryResponse {
        try await base.usageSummary(month: month)
    }
    func setBudgets(
        orgCents: Int64, bots: [Hearth_V1_BotBudget]
    ) async throws -> Hearth_V1_SummaryResponse {
        try await base.setBudgets(orgCents: orgCents, bots: bots)
    }
    func resume(botID: String) async throws -> Hearth_V1_SummaryResponse {
        try await base.resume(botID: botID)
    }
    func tiers() async throws -> Hearth_V1_TiersResponse { try await base.tiers() }
    func setTiers(_ tiers: [Hearth_V1_TierChoice]) async throws -> [Hearth_V1_TierChoice] {
        try await base.setTiers(tiers)
    }
    func listServiceModels(_ request: Hearth_V1_ListServiceModelsRequest) async throws -> [String] {
        try await base.listServiceModels(request)
    }
    func syncCredentials(_ records: [Hearth_V1_Credential]) async throws -> [Hearth_V1_Credential] {
        try await base.syncCredentials(records)
    }
    func exportCredential(name: String) async throws -> Hearth_V1_Credential {
        try await base.exportCredential(name: name)
    }
    func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    > {
        base.subscribe(after: sequence)
    }
}

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
    #expect(store.bots.map(\.name) == ["Dana"])
    await store.handle(botEvent(sam))
    #expect(store.bots.map(\.name) == ["Dana", "Sam"])
}

@MainActor
@Test func aChangedBotReplacesItsRowAndKeepsTheSelection() async {
    var renamed = Fixtures.bot
    renamed.status = .working
    let store = botList()
    await store.load()
    store.selection = Fixtures.bot.id
    await store.handle(botEvent(renamed))
    #expect(store.bots.count == 1)
    #expect(store.selectedBot?.status == .working)
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
    #expect(store.bots.count == 2)
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.bots.map(\.id) == [Fixtures.bot.id])
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
