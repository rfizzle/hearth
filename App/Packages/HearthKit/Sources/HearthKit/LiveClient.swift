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
    private let providers: Hearth_V1_ProviderServiceClient
    private let integrations: Hearth_V1_IntegrationServiceClient
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
        self.providers = Hearth_V1_ProviderServiceClient(client: protocolClient)
        self.integrations = Hearth_V1_IntegrationServiceClient(client: protocolClient)
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
        // The core answers not_found until onboarding has named the org.
        do {
            let response = try await org.getOrg(request: Hearth_V1_GetOrgRequest()).value()
            return response.hasOrg ? response.org : nil
        } catch let error as ConnectError where error.code == .notFound {
            return nil
        }
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

    public func send(conversationID: String, text: String) async throws -> Hearth_V1_SendResponse {
        let request = Hearth_V1_SendRequest.with {
            $0.conversationID = conversationID
            $0.text = text
        }
        return try await conversations.send(request: request).value()
    }

    public func cancel(runID: String) async throws {
        let request = Hearth_V1_CancelRequest.with { $0.runID = runID }
        _ = try await conversations.cancel(request: request).value()
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
    fileprivate func value() throws -> Output {
        try result.get()
    }
}
