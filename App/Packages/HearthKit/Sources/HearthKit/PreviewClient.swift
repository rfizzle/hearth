// PreviewClient answers from Fixtures instead of the core, for previews, the
// gallery and store tests. It keeps no state: a write answers as the core
// would and changes nothing, and subscribe replays Fixtures' scripted reply.

import Foundation

/// A client that never leaves the process.
public struct PreviewClient: HearthClient {
    /// The words streamHello cycles through.
    public static let words = ["Hello", "from", "the", "preview", "client."]

    /// The page size when listMessages is asked for the default.
    public static let defaultPageSize: Int32 = 50

    /// The latest run the DM reports, nil before its first message.
    public let latestRun: Hearth_V1_Run?

    /// A client whose DM reports `latestRun`.
    public init(latestRun: Hearth_V1_Run? = nil) {
        self.latestRun = latestRun
    }

    private var directMessage: Hearth_V1_Conversation {
        var conversation = Fixtures.dmConversation
        if let latestRun { conversation.latestRun = latestRun }
        return conversation
    }

    public func ping() async throws -> String {
        "preview"
    }

    public func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        let total = count > 0 ? Int(count) : 10
        return AsyncThrowingStream { continuation in
            for index in 0..<total {
                continuation.yield(Self.words[index % Self.words.count])
            }
            continuation.finish()
        }
    }

    public func getOrg() async throws -> Hearth_V1_Org? {
        Fixtures.org
    }

    public func updateOrg(name: String, workspacePath: String) async throws -> Hearth_V1_Org {
        .with {
            $0.name = name
            $0.workspacePath = workspacePath
        }
    }

    public func listTemplates() async throws -> [Hearth_V1_RoleTemplate] {
        Fixtures.templates
    }

    public func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot {
        var bot = Fixtures.bot
        bot.name = request.name
        bot.role = request.role
        bot.jobDescription = request.jobDescription
        bot.tier = request.tier
        bot.avatar = request.avatar
        bot.folderPath = Fixtures.workspacePath + "/" + request.name
        return bot
    }

    public func draftBot(
        _ request: Hearth_V1_DraftBotRequest
    ) async throws -> Hearth_V1_DraftBotResponse {
        Fixtures.draft
    }

    public func listBots() async throws -> [Hearth_V1_Bot] {
        [Fixtures.bot]
    }

    public func listConversations(botID: String) async throws -> [Hearth_V1_Conversation] {
        botID.isEmpty || botID == Fixtures.bot.id ? [directMessage] : []
    }

    public func listMessages(
        conversationID: String, beforeSeq: Int64, limit: Int32
    ) async throws -> Hearth_V1_ListMessagesResponse {
        let older = Fixtures.dmMessages.filter {
            $0.conversationID == conversationID && (beforeSeq == 0 || $0.seq < beforeSeq)
        }
        let size = Int(limit > 0 ? limit : Self.defaultPageSize)
        return .with {
            $0.messages = Array(older.suffix(size))
            $0.hasMore_p = older.count > size
        }
    }

    public func send(
        conversationID: String, text: String, attachmentPaths: [String]
    ) async throws -> Hearth_V1_SendResponse {
        var response = Fixtures.sendResponse
        response.message.conversationID = conversationID
        response.message.content = text
        response.run.conversationID = conversationID
        return response
    }

    public func cancel(runID: String) async throws {}

    public func getConversation(id: String) async throws -> Hearth_V1_Conversation {
        var conversation = directMessage
        conversation.id = id
        return conversation
    }

    public func retry(runID: String) async throws -> Hearth_V1_Run {
        var run = Fixtures.failedRun
        run.id = "run-retry"
        run.state = .queued
        run.failure = ""
        return run
    }

    public func listGrants(botID: String) async throws -> [Hearth_V1_Grant] {
        Fixtures.grants
    }

    public func setGrants(
        botID: String, _ grants: [Hearth_V1_Grant]
    ) async throws -> [Hearth_V1_Grant] {
        grants
    }

    public func listInbox(includeClosed: Bool) async throws -> [Hearth_V1_InboxItem] {
        Fixtures.inbox.filter { includeClosed || $0.state == .open }
    }

    public func decide(
        itemID: String, _ decision: Hearth_V1_Decision, reason: String
    ) async throws -> Hearth_V1_DecideResponse {
        var item = Fixtures.inbox.first { $0.id == itemID } ?? Fixtures.approval
        item.state = .closed
        item.decision = decision
        item.outcome = decision == .deny ? "Denied" : "Allowed"
        return .with {
            $0.item = item
            $0.run = Fixtures.workingRun
        }
    }

    public func answer(
        itemID: String, _ text: String, value: Hearth_V1_WidgetAnswer?
    ) async throws -> Hearth_V1_AnswerResponse {
        var item = Fixtures.inbox.first { $0.id == itemID } ?? Fixtures.question
        item.state = .closed
        item.answer = text
        if let value { item.answerValue = value }
        item.outcome = "Answered"
        return .with {
            $0.item = item
            $0.run = Fixtures.workingRun
        }
    }

    public func acknowledge(itemID: String) async throws -> Hearth_V1_InboxItem {
        var item = Fixtures.inbox.first { $0.id == itemID } ?? Fixtures.failure
        item.state = .closed
        item.outcome = "Acknowledged"
        return item
    }

    public func listAccounts() async throws -> [Hearth_V1_ProviderAccount] {
        Fixtures.accounts
    }

    public func setAPIKey(
        provider: Hearth_V1_Provider, key: String
    ) async throws -> Hearth_V1_ProviderAccount {
        .with {
            $0.provider = provider
            $0.state = .signedIn
            $0.label = String(key.suffix(4))
        }
    }

    public func clearCredential(provider: Hearth_V1_Provider) async throws {}

    public func saveService(
        _ request: Hearth_V1_SetAPIKeyRequest
    ) async throws -> Hearth_V1_ProviderAccount {
        .with {
            $0.provider = .openaiCompatible
            $0.state = .signedIn
            $0.profile =
                request.profile.isEmpty
                ? Fixtures.profileID(for: request.displayName) : request.profile
            $0.displayName = request.displayName
            $0.baseURL = request.baseURL
            $0.defaultModel = request.defaultModel
            $0.label = String(request.key.suffix(4))
        }
    }

    public func removeService(profile: String) async throws {}

    public func listKnownServices() async throws -> [Hearth_V1_KnownService] {
        Fixtures.knownServices
    }

    public func startChatGPTLogin() async throws -> URL {
        Fixtures.authorizeURL
    }

    public func listIntegrations() async throws -> [Hearth_V1_IntegrationAccount] {
        Fixtures.integrations
    }

    public func setIntegrationKey(
        _ integration: Hearth_V1_Integration, key: String
    ) async throws -> Hearth_V1_IntegrationAccount {
        var account =
            Fixtures.integrations.first { $0.integration == integration }
            ?? .with { $0.integration = integration }
        account.configured = true
        account.label = String(key.suffix(4))
        return account
    }

    public func clearIntegration(_ integration: Hearth_V1_Integration) async throws {}

    public func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    > {
        AsyncThrowingStream { continuation in
            for event in Fixtures.replyEvents where event.sequence > sequence {
                continuation.yield(event)
            }
            continuation.finish()
        }
    }
}
