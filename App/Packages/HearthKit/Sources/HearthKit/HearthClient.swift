// HearthClient is the one seam between the app and the core: stores take it in
// their initialiser, LiveClient speaks to hearthd and PreviewClient serves
// fixtures to previews, the gallery and store tests. The generated proto types
// are the models; nothing here mirrors them.

import Foundation

/// What a store may ask of the core.
public protocol HearthClient: Sendable {
    /// The core's version string.
    func ping() async throws -> String
    /// Streams `count` words from the core; 0 asks for the core's default.
    func streamHello(count: Int32) -> AsyncThrowingStream<String, Error>

    /// The org, or nil while onboarding has not named it.
    func getOrg() async throws -> Hearth_V1_Org?
    /// Names the org and sets its Hearth folder; the first call creates both.
    func updateOrg(name: String, workspacePath: String) async throws -> Hearth_V1_Org

    /// The role templates a new bot can start from.
    func listTemplates() async throws -> [Hearth_V1_RoleTemplate]
    /// Creates a bot, its folder and its direct-message conversation.
    func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot
    /// Every bot in the org.
    func listBots() async throws -> [Hearth_V1_Bot]

    /// The conversations `botID` is in; an empty id lists every conversation.
    func listConversations(botID: String) async throws -> [Hearth_V1_Conversation]
    /// One page of messages below `beforeSeq` (0 for the newest), ascending by
    /// seq; a `limit` of 0 asks for the core's default page.
    func listMessages(conversationID: String, beforeSeq: Int64, limit: Int32) async throws
        -> Hearth_V1_ListMessagesResponse
    /// Stores the owner's message and starts one run; the reply arrives on
    /// `subscribe`.
    func send(conversationID: String, text: String) async throws -> Hearth_V1_SendResponse
    /// Stops a run, keeping what the bot had written so far.
    func cancel(runID: String) async throws

    /// Every provider and whether it has a credential; never the credential.
    func listAccounts() async throws -> [Hearth_V1_ProviderAccount]
    /// Stores an API key for `provider`.
    func setAPIKey(provider: Hearth_V1_Provider, key: String) async throws
        -> Hearth_V1_ProviderAccount
    /// Forgets the credential for `provider`.
    func clearCredential(provider: Hearth_V1_Provider) async throws
    /// Starts the ChatGPT sign-in and returns the page to open in the browser;
    /// the result arrives on `subscribe` as an account change.
    func startChatGPTLogin() async throws -> URL

    /// The services that are not model providers, such as web search, and
    /// whether each has a key; never the key.
    func listIntegrations() async throws -> [Hearth_V1_IntegrationAccount]
    /// Stores the key for `integration`, switching its tool on.
    func setIntegrationKey(_ integration: Hearth_V1_Integration, key: String) async throws
        -> Hearth_V1_IntegrationAccount
    /// Forgets the key for `integration`, switching its tool off.
    func clearIntegration(_ integration: Hearth_V1_Integration) async throws

    /// Every change the core makes after `sequence` (0 for live events only),
    /// in order. The stream finishes with the error that ended it.
    func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    >
}

/// A reply from the core the client cannot use.
public enum HearthClientError: Error, Equatable {
    /// The sign-in page the core returned is not a URL.
    case invalidAuthorizeURL(String)
}
