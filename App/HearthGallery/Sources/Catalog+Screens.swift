// The screens, each over PreviewClient and the fixtures. A store loads in the
// background, so each is nudged to the state its card names as the catalogue
// is built; the screens' own tasks then find their data already there.

import Foundation
import HearthFeatures
import HearthKit
import SwiftUI

extension Catalog {
    static var screens: [GalleryPage] {
        let package = "HearthFeatures"
        return [
            GalleryPage(
                package: package, name: "OnboardingScreen",
                states: [
                    onboarding(.welcome, name: "welcome"),
                    onboarding(.signIn, name: "sign in"),
                    onboarding(.nameAndFolder, name: "name and folder"),
                ]),
            GalleryPage(
                package: package, name: "ConversationScreen",
                states: [
                    conversationEmpty(), conversationAboutOpen(), conversationMidStream(),
                    conversationSummaryOpen(), conversationWaiting(),
                ] + projectConversationStates),
            GalleryPage(
                package: package, name: "MenuBarExtra",
                states: [menuBarFilled(), menuBarEmpty(), menuBarReplyStreaming()]),
        ]
    }

    /// A client that employs nobody, for the empty team.
    static var noBotsClient: any HearthClient { NoBotsClient() }

    private static func onboarding(_ step: OnboardingStore.Step, name: String) -> GalleryState {
        let client = PreviewClient()
        let accounts = AccountsStore(client: client, relay: EventRelay(client: client))
        let store = OnboardingStore(client: client, accounts: accounts)
        store.path = Fixtures.workspacePath
        store.name = step == .nameAndFolder ? Fixtures.org.name : ""
        Task {
            await accounts.load()
            for _ in 0..<step.rawValue { store.next() }
        }
        return GalleryState(
            name: name, width: 760,
            view: AnyView(OnboardingScreen(store: store).frame(height: 600)))
    }

    /// The reply has opened and two words have arrived; the stream stays open.
    private static func conversationMidStream() -> GalleryState {
        let client = PreviewClient()
        let arrived = Array(Fixtures.replyEvents.prefix(5))
        let relay = EventRelay(subscribe: { after in
            AsyncThrowingStream { continuation in
                for event in arrived where event.sequence > after { continuation.yield(event) }
                // Left open on purpose: the run is still working.
                Task {
                    try? await Task.sleep(for: .seconds(86_400))
                    continuation.finish()
                }
            }
        })
        let store = ConversationStore(
            client: client, relay: relay, conversationID: Fixtures.dmConversation.id)
        Task {
            await store.load()
            relay.start()
        }
        return GalleryState(
            name: "mid-stream", width: 560,
            view: AnyView(
                ConversationScreen(store: store, bot: Fixtures.bot).frame(height: 360)))
    }
}

extension Catalog {
    /// The DM with the earlier-summary row open.
    fileprivate static func conversationSummaryOpen() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let store = ConversationStore(
            client: client, relay: relay, conversationID: Fixtures.dmConversation.id)
        return GalleryState(
            name: "earlier summary open", width: 560,
            view: AnyView(
                ConversationScreen(store: store, bot: Fixtures.bot, summaryOpen: true)
                    .frame(height: 700)))
    }

    /// The DM with the "About" header open over the transcript.
    fileprivate static func conversationAboutOpen() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let store = ConversationStore(
            client: client, relay: relay, conversationID: Fixtures.dmConversation.id)
        store.aboutOpen = true
        return GalleryState(
            name: "about open", width: 560,
            view: AnyView(ConversationScreen(store: store, bot: Fixtures.bot).frame(height: 760)))
    }

    /// A conversation nobody has written in yet: the id matches no messages.
    fileprivate static func conversationEmpty() -> GalleryState {
        let client = PreviewClient()
        let relay = EventRelay(client: client)
        let store = ConversationStore(client: client, relay: relay, conversationID: "empty")
        return GalleryState(
            name: "empty", width: 560,
            view: AnyView(
                ConversationScreen(store: store, bot: Fixtures.chiefOfStaff).frame(height: 760)))
    }
}

private typealias Page = Hearth_V1_ListMessagesResponse
private typealias Account = Hearth_V1_ProviderAccount
private typealias Key = Hearth_V1_IntegrationAccount

/// A client that serves the fixtures but employs nobody, so nothing waits on
/// the owner either: the empty list and the quiet menu bar.
private struct NoBotsClient: HearthClient {
    private let preview = PreviewClient()

    func ping() async throws -> String { try await preview.ping() }
    func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        preview.streamHello(count: count)
    }
    func getOrg() async throws -> Hearth_V1_Org? { try await preview.getOrg() }
    func updateOrg(name: String, workspacePath: String) async throws -> Hearth_V1_Org {
        try await preview.updateOrg(name: name, workspacePath: workspacePath)
    }
    func listTemplates() async throws -> [Hearth_V1_RoleTemplate] {
        try await preview.listTemplates()
    }
    func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot {
        try await preview.createBot(request)
    }
    func draftBot(
        _ request: Hearth_V1_DraftBotRequest
    ) async throws -> Hearth_V1_DraftBotResponse {
        try await preview.draftBot(request)
    }
    func checkBot(_ request: Hearth_V1_CheckBotRequest) async throws -> [Hearth_V1_Concern] {
        try await preview.checkBot(request)
    }
    func listBots() async throws -> [Hearth_V1_Bot] { [] }
    func listConversations(botID: String) async throws -> [Hearth_V1_Conversation] {
        try await preview.listConversations(botID: botID)
    }
    func listMessages(conversationID: String, beforeSeq: Int64, limit: Int32) async throws -> Page {
        try await preview.listMessages(
            conversationID: conversationID, beforeSeq: beforeSeq, limit: limit)
    }
    func send(
        conversationID: String, text: String, attachmentPaths: [String]
    ) async throws -> Hearth_V1_SendResponse {
        try await preview.send(
            conversationID: conversationID, text: text, attachmentPaths: attachmentPaths)
    }
    func cancel(runID: String) async throws { try await preview.cancel(runID: runID) }
    func getConversation(id: String) async throws -> Hearth_V1_Conversation {
        try await preview.getConversation(id: id)
    }
    func retry(runID: String) async throws -> Hearth_V1_Run {
        try await preview.retry(runID: runID)
    }
    func listProjects(includeArchived: Bool) async throws -> [Hearth_V1_Project] {
        try await preview.listProjects(includeArchived: includeArchived)
    }
    func draftProject(description: String) async throws -> Hearth_V1_DraftProjectResponse {
        try await preview.draftProject(description: description)
    }
    func checkProject(
        _ request: Hearth_V1_CheckProjectRequest
    ) async throws -> [Hearth_V1_Concern] {
        try await preview.checkProject(request)
    }
    func createProject(
        _ request: Hearth_V1_CreateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await preview.createProject(request)
    }
    func updateProject(
        _ request: Hearth_V1_UpdateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await preview.updateProject(request)
    }
    func markProjectDone(id: String) async throws -> Hearth_V1_Project {
        try await preview.markProjectDone(id: id)
    }
    func archiveProject(id: String) async throws -> Hearth_V1_Project {
        try await preview.archiveProject(id: id)
    }
    func listMeasures() async throws -> [Hearth_V1_MeasureValue] {
        try await preview.listMeasures()
    }
    func readMeasures(ids: [String]) async throws -> [Hearth_V1_MeasureValue] {
        try await preview.readMeasures(ids: ids)
    }
    func listDeliverableTemplates() async throws -> [Hearth_V1_DeliverableTemplate] {
        try await preview.listDeliverableTemplates()
    }
    func saveDeliverableTemplate(json: String) async throws -> Hearth_V1_DeliverableTemplate {
        try await preview.saveDeliverableTemplate(json: json)
    }
    func deleteDeliverableTemplate(id: String) async throws {
        try await preview.deleteDeliverableTemplate(id: id)
    }
    func listGrants(botID: String) async throws -> [Hearth_V1_Grant] {
        try await preview.listGrants(botID: botID)
    }
    func setGrants(botID: String, _ grants: [Hearth_V1_Grant]) async throws -> [Hearth_V1_Grant] {
        try await preview.setGrants(botID: botID, grants)
    }
    func listInbox(includeClosed: Bool) async throws -> [Hearth_V1_InboxItem] { [] }
    func decide(
        itemID: String, _ decision: Hearth_V1_Decision, reason: String
    ) async throws -> Hearth_V1_DecideResponse {
        try await preview.decide(itemID: itemID, decision, reason: reason)
    }
    func answer(
        itemID: String, _ text: String, value: Hearth_V1_WidgetAnswer?
    ) async throws -> Hearth_V1_AnswerResponse {
        try await preview.answer(itemID: itemID, text, value: value)
    }
    func acknowledge(itemID: String) async throws -> Hearth_V1_InboxItem {
        try await preview.acknowledge(itemID: itemID)
    }
    func listAccounts() async throws -> [Hearth_V1_ProviderAccount] {
        try await preview.listAccounts()
    }
    func setAPIKey(provider: Hearth_V1_Provider, key: String) async throws -> Account {
        try await preview.setAPIKey(provider: provider, key: key)
    }
    func clearCredential(provider: Hearth_V1_Provider) async throws {
        try await preview.clearCredential(provider: provider)
    }
    func saveService(
        _ request: Hearth_V1_SetAPIKeyRequest
    ) async throws -> Hearth_V1_ProviderAccount {
        try await preview.saveService(request)
    }
    func removeService(profile: String) async throws {
        try await preview.removeService(profile: profile)
    }
    func listKnownServices() async throws -> [Hearth_V1_KnownService] {
        try await preview.listKnownServices()
    }
    func startChatGPTLogin() async throws -> URL { try await preview.startChatGPTLogin() }
    func listIntegrations() async throws -> [Hearth_V1_IntegrationAccount] {
        try await preview.listIntegrations()
    }
    func setIntegrationKey(_ integration: Hearth_V1_Integration, key: String) async throws -> Key {
        try await preview.setIntegrationKey(integration, key: key)
    }
    func clearIntegration(_ integration: Hearth_V1_Integration) async throws {
        try await preview.clearIntegration(integration)
    }
    func updateBot(_ request: Hearth_V1_UpdateBotRequest) async throws -> Hearth_V1_Bot {
        try await preview.updateBot(request)
    }
    func listNotes(botID: String) async throws -> Hearth_V1_ListNotesResponse {
        try await preview.listNotes(botID: botID)
    }
    func editNote(id: String, text: String) async throws -> Hearth_V1_Note {
        try await preview.editNote(id: id, text: text)
    }
    func deleteNote(id: String) async throws { try await preview.deleteNote(id: id) }
    func pinNote(id: String, pinned: Bool) async throws -> Hearth_V1_Note {
        try await preview.pinNote(id: id, pinned: pinned)
    }
    func listPages(botID: String) async throws -> Hearth_V1_ListPagesResponse {
        try await preview.listPages(botID: botID)
    }
    func putPage(_ request: Hearth_V1_PutPageRequest) async throws -> Hearth_V1_PutPageResponse {
        try await preview.putPage(request)
    }
    func deletePage(id: String) async throws -> Hearth_V1_ContextMeter {
        try await preview.deletePage(id: id)
    }
    func usageSummary(month: String) async throws -> Hearth_V1_SummaryResponse {
        try await preview.usageSummary(month: month)
    }
    func setBudgets(
        orgCents: Int64, bots: [Hearth_V1_BotBudget]
    ) async throws -> Hearth_V1_SummaryResponse {
        try await preview.setBudgets(orgCents: orgCents, bots: bots)
    }
    func resume(botID: String) async throws -> Hearth_V1_SummaryResponse {
        try await preview.resume(botID: botID)
    }
    func tiers() async throws -> Hearth_V1_TiersResponse { try await preview.tiers() }
    func setTiers(_ tiers: [Hearth_V1_TierChoice]) async throws -> [Hearth_V1_TierChoice] {
        try await preview.setTiers(tiers)
    }
    func listServiceModels(_ request: Hearth_V1_ListServiceModelsRequest) async throws -> [String] {
        try await preview.listServiceModels(request)
    }
    func syncCredentials(_ records: [Hearth_V1_Credential]) async throws -> [Hearth_V1_Credential] {
        try await preview.syncCredentials(records)
    }
    func exportCredential(name: String) async throws -> Hearth_V1_Credential {
        try await preview.exportCredential(name: name)
    }
    func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    > {
        preview.subscribe(after: sequence)
    }
}
