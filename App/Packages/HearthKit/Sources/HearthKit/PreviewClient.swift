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

    public init() {}

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
        botID.isEmpty || botID == Fixtures.bot.id ? [Fixtures.dmConversation] : []
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

    public func send(conversationID: String, text: String) async throws -> Hearth_V1_SendResponse {
        var response = Fixtures.sendResponse
        response.message.conversationID = conversationID
        response.message.content = text
        response.run.conversationID = conversationID
        return response
    }

    public func cancel(runID: String) async throws {}

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
