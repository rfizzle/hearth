// LiveClient speaks the hearth.v1 contract to hearthd over loopback, with the
// per-launch token on every request. The token rides in the URLSession
// configuration's additional headers, so no interceptor is needed. Every
// service shares one protocol client; the unary calls unwrap their result and
// the two server streams finish with the error that ended them.

import Connect
import Foundation

/// The client that talks to a running hearthd.
public final class LiveClient: HearthClient {
    private let hearth: Hearth_V1_HearthServiceClient
    private let org: Hearth_V1_OrgServiceClient
    private let bots: Hearth_V1_BotServiceClient
    private let conversations: Hearth_V1_ConversationServiceClient
    private let inbox: Hearth_V1_InboxServiceClient
    private let providers: Hearth_V1_ProviderServiceClient
    private let handbook: Hearth_V1_HandbookServiceClient
    private let usage: Hearth_V1_UsageServiceClient
    private let integrations: Hearth_V1_IntegrationServiceClient
    let projects: Hearth_V1_ProjectServiceClient
    private let events: Hearth_V1_EventServiceClient

    /// Connects to hearthd at `baseURL` (for example `http://127.0.0.1:52011`),
    /// sending `token` as a bearer token on every request.
    public init(baseURL: URL, token: String) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpAdditionalHeaders = ["Authorization": "Bearer \(token)"]
        let protocolClient = ProtocolClient(
            httpClient: URLSessionHTTPClient(configuration: configuration),
            config: ProtocolClientConfig(
                host: baseURL.absoluteString,
                networkProtocol: .connect,
                codec: ProtoCodec()
            )
        )
        self.hearth = Hearth_V1_HearthServiceClient(client: protocolClient)
        self.org = Hearth_V1_OrgServiceClient(client: protocolClient)
        self.bots = Hearth_V1_BotServiceClient(client: protocolClient)
        self.conversations = Hearth_V1_ConversationServiceClient(client: protocolClient)
        self.inbox = Hearth_V1_InboxServiceClient(client: protocolClient)
        self.providers = Hearth_V1_ProviderServiceClient(client: protocolClient)
        self.handbook = Hearth_V1_HandbookServiceClient(client: protocolClient)
        self.usage = Hearth_V1_UsageServiceClient(client: protocolClient)
        self.integrations = Hearth_V1_IntegrationServiceClient(client: protocolClient)
        self.projects = Hearth_V1_ProjectServiceClient(client: protocolClient)
        self.events = Hearth_V1_EventServiceClient(client: protocolClient)
    }

    public func ping() async throws -> String {
        try await hearth.ping(request: Hearth_V1_PingRequest()).value().version
    }

    public func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        let request = Hearth_V1_HelloRequest.with { $0.count = count }
        return serverStream(hearth.streamHello(), request: request) { $0.word }
    }

    public func getOrg() async throws -> Hearth_V1_Org? {
        // Until onboarding has named the org the core answers with none.
        let response = try await org.getOrg(request: Hearth_V1_GetOrgRequest()).value()
        return response.hasOrg ? response.org : nil
    }

    public func updateOrg(name: String, workspacePath: String) async throws -> Hearth_V1_Org {
        let request = Hearth_V1_UpdateOrgRequest.with {
            $0.name = name
            $0.workspacePath = workspacePath
        }
        return try await org.updateOrg(request: request).value().org
    }

    public func listTemplates() async throws -> [Hearth_V1_RoleTemplate] {
        try await bots.listTemplates(request: Hearth_V1_ListTemplatesRequest()).value().templates
    }

    public func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot {
        try await bots.createBot(request: request).value().bot
    }

    public func draftBot(
        _ request: Hearth_V1_DraftBotRequest
    ) async throws -> Hearth_V1_DraftBotResponse {
        try await bots.draftBot(request: request).value()
    }

    public func listBots() async throws -> [Hearth_V1_Bot] {
        try await bots.listBots(request: Hearth_V1_ListBotsRequest()).value().bots
    }

    public func listConversations(botID: String) async throws -> [Hearth_V1_Conversation] {
        let request = Hearth_V1_ListConversationsRequest.with { $0.botID = botID }
        return try await conversations.listConversations(request: request).value().conversations
    }

    public func listMessages(
        conversationID: String, beforeSeq: Int64, limit: Int32
    ) async throws -> Hearth_V1_ListMessagesResponse {
        let request = Hearth_V1_ListMessagesRequest.with {
            $0.conversationID = conversationID
            $0.beforeSeq = beforeSeq
            $0.limit = limit
        }
        return try await conversations.listMessages(request: request).value()
    }

    public func send(
        conversationID: String, text: String, attachmentPaths: [String]
    ) async throws -> Hearth_V1_SendResponse {
        let request = Hearth_V1_SendRequest.with {
            $0.conversationID = conversationID
            $0.text = text
            $0.attachmentPaths = attachmentPaths
        }
        return try await conversations.send(request: request).value()
    }

    public func cancel(runID: String) async throws {
        let request = Hearth_V1_CancelRequest.with { $0.runID = runID }
        _ = try await conversations.cancel(request: request).value()
    }

    public func getConversation(id: String) async throws -> Hearth_V1_Conversation {
        let request = Hearth_V1_GetConversationRequest.with { $0.id = id }
        return try await conversations.getConversation(request: request).value().conversation
    }

    public func retry(runID: String) async throws -> Hearth_V1_Run {
        let request = Hearth_V1_RetryRequest.with { $0.runID = runID }
        return try await conversations.retry(request: request).value().run
    }

    public func listGrants(botID: String) async throws -> [Hearth_V1_Grant] {
        let request = Hearth_V1_ListGrantsRequest.with { $0.botID = botID }
        return try await bots.listGrants(request: request).value().grants
    }

    public func setGrants(
        botID: String, _ grants: [Hearth_V1_Grant]
    ) async throws -> [Hearth_V1_Grant] {
        let request = Hearth_V1_SetGrantsRequest.with {
            $0.botID = botID
            $0.grants = grants
        }
        return try await bots.setGrants(request: request).value().grants
    }

    public func listInbox(includeClosed: Bool) async throws -> [Hearth_V1_InboxItem] {
        let request = Hearth_V1_ListInboxRequest.with { $0.includeClosed = includeClosed }
        return try await inbox.listInbox(request: request).value().items
    }

    public func decide(
        itemID: String, _ decision: Hearth_V1_Decision, reason: String
    ) async throws -> Hearth_V1_DecideResponse {
        let request = Hearth_V1_DecideRequest.with {
            $0.itemID = itemID
            $0.decision = decision
            $0.reason = reason
        }
        return try await inbox.decide(request: request).value()
    }

    public func answer(
        itemID: String, _ text: String, value: Hearth_V1_WidgetAnswer?
    ) async throws -> Hearth_V1_AnswerResponse {
        let request = Hearth_V1_AnswerRequest.with {
            $0.itemID = itemID
            $0.answer = text
            if let value { $0.value = value }
        }
        return try await inbox.answer(request: request).value()
    }

    public func acknowledge(itemID: String) async throws -> Hearth_V1_InboxItem {
        let request = Hearth_V1_AcknowledgeRequest.with { $0.itemID = itemID }
        return try await inbox.acknowledge(request: request).value().item
    }

    public func listAccounts() async throws -> [Hearth_V1_ProviderAccount] {
        try await providers.listAccounts(request: Hearth_V1_ListAccountsRequest()).value().accounts
    }

    public func setAPIKey(
        provider: Hearth_V1_Provider, key: String
    ) async throws -> Hearth_V1_ProviderAccount {
        let request = Hearth_V1_SetAPIKeyRequest.with {
            $0.provider = provider
            $0.key = key
        }
        return try await providers.setApikey(request: request).value().account
    }

    public func clearCredential(provider: Hearth_V1_Provider) async throws {
        let request = Hearth_V1_ClearCredentialRequest.with { $0.provider = provider }
        _ = try await providers.clearCredential(request: request).value()
    }

    public func saveService(
        _ request: Hearth_V1_SetAPIKeyRequest
    ) async throws -> Hearth_V1_ProviderAccount {
        var request = request
        request.provider = .openaiCompatible
        return try await providers.setApikey(request: request).value().account
    }

    public func removeService(profile: String) async throws {
        let request = Hearth_V1_ClearCredentialRequest.with {
            $0.provider = .openaiCompatible
            $0.profile = profile
        }
        _ = try await providers.clearCredential(request: request).value()
    }

    public func listKnownServices() async throws -> [Hearth_V1_KnownService] {
        let request = Hearth_V1_ListKnownServicesRequest()
        return try await providers.listKnownServices(request: request).value().services
    }

    public func startChatGPTLogin() async throws -> URL {
        let request = Hearth_V1_StartChatGPTLoginRequest()
        let page = try await providers.startChatGptlogin(request: request).value().authorizeURL
        guard let url = URL(string: page) else {
            throw HearthClientError.invalidAuthorizeURL(page)
        }
        return url
    }

    public func listIntegrations() async throws -> [Hearth_V1_IntegrationAccount] {
        let request = Hearth_V1_ListIntegrationsRequest()
        return try await integrations.listIntegrations(request: request).value().integrations
    }

    public func setIntegrationKey(
        _ integration: Hearth_V1_Integration, key: String
    ) async throws -> Hearth_V1_IntegrationAccount {
        let request = Hearth_V1_SetIntegrationKeyRequest.with {
            $0.integration = integration
            $0.key = key
        }
        return try await integrations.setIntegrationKey(request: request).value().integration
    }

    public func clearIntegration(_ integration: Hearth_V1_Integration) async throws {
        let request = Hearth_V1_ClearIntegrationRequest.with { $0.integration = integration }
        _ = try await integrations.clearIntegration(request: request).value()
    }

    public func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    > {
        let request = Hearth_V1_SubscribeRequest.with { $0.afterSequence = sequence }
        return serverStream(events.subscribe(), request: request) { $0 }
    }

    /// Sends `request` on a server stream and yields each reply through
    /// `transform`, finishing with the stream's error, if any.
    private func serverStream<Input: Sendable, Output, Element: Sendable>(
        _ stream: any ServerOnlyAsyncStreamInterface<Input, Output>,
        request: Input,
        transform: @escaping @Sendable (Output) -> Element
    ) -> AsyncThrowingStream<Element, Error> {
        AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    try stream.send(request)
                } catch {
                    continuation.finish(throwing: error)
                    return
                }
                for await result in stream.results() {
                    switch result {
                    case .headers:
                        continue
                    case .message(let reply):
                        continuation.yield(transform(reply))
                    case .complete(_, let error, _):
                        continuation.finish(throwing: error)
                        return
                    }
                }
                continuation.finish()
            }
            continuation.onTermination = { _ in
                task.cancel()
                stream.cancel()
            }
        }
    }
}

extension ResponseMessage {
    /// The reply, or the error the call failed with.
    func value() throws -> Output {
        try result.get()
    }
}

// The slice B calls, in an extension to keep the class body short.
extension LiveClient {
    public func updateBot(_ request: Hearth_V1_UpdateBotRequest) async throws -> Hearth_V1_Bot {
        try await bots.updateBot(request: request).value().bot
    }

    public func listNotes(botID: String) async throws -> Hearth_V1_ListNotesResponse {
        try await bots.listNotes(request: .with { $0.botID = botID }).value()
    }

    public func editNote(id: String, text: String) async throws -> Hearth_V1_Note {
        let request = Hearth_V1_EditNoteRequest.with {
            $0.id = id
            $0.text = text
        }
        return try await bots.editNote(request: request).value().note
    }

    public func deleteNote(id: String) async throws {
        _ = try await bots.deleteNote(request: .with { $0.id = id }).value()
    }

    public func pinNote(id: String, pinned: Bool) async throws -> Hearth_V1_Note {
        let request = Hearth_V1_PinNoteRequest.with {
            $0.id = id
            $0.pinned = pinned
        }
        return try await bots.pinNote(request: request).value().note
    }

    public func listPages(botID: String) async throws -> Hearth_V1_ListPagesResponse {
        try await handbook.listPages(request: .with { $0.botID = botID }).value()
    }

    public func putPage(
        _ request: Hearth_V1_PutPageRequest
    ) async throws -> Hearth_V1_PutPageResponse {
        try await handbook.putPage(request: request).value()
    }

    public func deletePage(id: String) async throws -> Hearth_V1_ContextMeter {
        try await handbook.deletePage(request: .with { $0.id = id }).value().meter
    }

    public func usageSummary(month: String) async throws -> Hearth_V1_SummaryResponse {
        try await usage.summary(request: .with { $0.month = month }).value()
    }

    public func setBudgets(
        orgCents: Int64, bots: [Hearth_V1_BotBudget]
    ) async throws -> Hearth_V1_SummaryResponse {
        let request = Hearth_V1_SetBudgetsRequest.with {
            $0.orgCents = orgCents
            $0.bots = bots
        }
        return try await usage.setBudgets(request: request).value().summary
    }

    public func resume(botID: String) async throws -> Hearth_V1_SummaryResponse {
        try await usage.resume(request: .with { $0.botID = botID }).value().summary
    }

    public func tiers() async throws -> Hearth_V1_TiersResponse {
        try await providers.tiers(request: Hearth_V1_TiersRequest()).value()
    }

    public func setTiers(_ tiers: [Hearth_V1_TierChoice]) async throws -> [Hearth_V1_TierChoice] {
        try await providers.setTiers(request: .with { $0.tiers = tiers }).value().tiers
    }

    public func listServiceModels(
        _ request: Hearth_V1_ListServiceModelsRequest
    ) async throws -> [String] {
        try await providers.listServiceModels(request: request).value().models
    }

    public func syncCredentials(
        _ records: [Hearth_V1_Credential]
    ) async throws -> [Hearth_V1_Credential] {
        try await providers.syncCredentials(request: .with { $0.records = records }).value()
            .recordsToStore
    }

    public func exportCredential(name: String) async throws -> Hearth_V1_Credential {
        try await providers.exportCredential(request: .with { $0.name = name }).value().record
    }
}
