// A client with nobody hired yet, for the store tests that start from an empty
// org.

import Foundation
import HearthKit

/// A client with nobody hired yet; everything else is PreviewClient's.
struct EmptyOrgClient: HearthClient {
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
    func checkBot(_ request: Hearth_V1_CheckBotRequest) async throws -> [Hearth_V1_Concern] {
        try await base.checkBot(request)
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
    func listProjects(includeArchived: Bool) async throws -> [Hearth_V1_Project] {
        try await base.listProjects(includeArchived: includeArchived)
    }
    func draftProject(description: String) async throws -> Hearth_V1_DraftProjectResponse {
        try await base.draftProject(description: description)
    }
    func checkProject(
        _ request: Hearth_V1_CheckProjectRequest
    ) async throws -> [Hearth_V1_Concern] {
        try await base.checkProject(request)
    }
    func createProject(
        _ request: Hearth_V1_CreateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await base.createProject(request)
    }
    func updateProject(
        _ request: Hearth_V1_UpdateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await base.updateProject(request)
    }
    func markProjectDone(id: String) async throws -> Hearth_V1_Project {
        try await base.markProjectDone(id: id)
    }
    func archiveProject(id: String) async throws -> Hearth_V1_Project {
        try await base.archiveProject(id: id)
    }
    func listMeasures() async throws -> [Hearth_V1_MeasureValue] {
        try await base.listMeasures()
    }
    func readMeasures(ids: [String]) async throws -> [Hearth_V1_MeasureValue] {
        try await base.readMeasures(ids: ids)
    }
    func listDeliverableTemplates() async throws -> [Hearth_V1_DeliverableTemplate] {
        try await base.listDeliverableTemplates()
    }
    func saveDeliverableTemplate(json: String) async throws -> Hearth_V1_DeliverableTemplate {
        try await base.saveDeliverableTemplate(json: json)
    }
    func deleteDeliverableTemplate(id: String) async throws {
        try await base.deleteDeliverableTemplate(id: id)
    }
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
