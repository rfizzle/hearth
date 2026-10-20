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
            GalleryPage(package: package, name: "HireSheet", states: [hireWithTemplate()]),
            GalleryPage(
                package: package, name: "BotList",
                states: [
                    botList(PreviewClient(), name: "one bot"),
                    botList(NoBotsClient(), name: "none"),
                ]),
            GalleryPage(
                package: package, name: "ConversationScreen", states: [conversationMidStream()]),
        ]
    }

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
            name: name, width: 560, view: AnyView(OnboardingScreen(store: store)))
    }

    private static func hireWithTemplate() -> GalleryState {
        let store = HireStore(client: PreviewClient())
        Task {
            await store.load()
            if let template = store.templates.first { store.pick(template) }
        }
        return GalleryState(
            name: "template picked", width: 560,
            view: AnyView(HireSheet(store: store, onHired: { _ in }, onCancel: {})))
    }

    private static func botList(_ client: any HearthClient, name: String) -> GalleryState {
        let store = BotListStore(client: client, events: AsyncStream { $0.finish() })
        store.selection = Fixtures.bot.id
        return GalleryState(
            name: name, width: 280,
            view: AnyView(BotList(store: store, onHire: {}).frame(height: 240)))
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
                ConversationScreen(store: store, bot: Fixtures.bot).frame(height: 520)))
    }
}

private typealias Page = Hearth_V1_ListMessagesResponse
private typealias Account = Hearth_V1_ProviderAccount
private typealias Key = Hearth_V1_IntegrationAccount

/// A client that serves the fixtures but employs nobody, for the empty list.
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
    func listBots() async throws -> [Hearth_V1_Bot] { [] }
    func listConversations(botID: String) async throws -> [Hearth_V1_Conversation] {
        try await preview.listConversations(botID: botID)
    }
    func listMessages(conversationID: String, beforeSeq: Int64, limit: Int32) async throws -> Page {
        try await preview.listMessages(
            conversationID: conversationID, beforeSeq: beforeSeq, limit: limit)
    }
    func send(conversationID: String, text: String) async throws -> Hearth_V1_SendResponse {
        try await preview.send(conversationID: conversationID, text: text)
    }
    func cancel(runID: String) async throws { try await preview.cancel(runID: runID) }
    func listAccounts() async throws -> [Hearth_V1_ProviderAccount] {
        try await preview.listAccounts()
    }
    func setAPIKey(provider: Hearth_V1_Provider, key: String) async throws -> Account {
        try await preview.setAPIKey(provider: provider, key: key)
    }
    func clearCredential(provider: Hearth_V1_Provider) async throws {
        try await preview.clearCredential(provider: provider)
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
    func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    > {
        preview.subscribe(after: sequence)
    }
}
