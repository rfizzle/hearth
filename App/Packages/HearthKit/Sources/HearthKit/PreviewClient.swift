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

    /// How the credential calls answer; without one they answer with nothing.
    public let credentials: CredentialScript?

    /// A client whose DM reports `latestRun`.
    public init(latestRun: Hearth_V1_Run? = nil, credentials: CredentialScript? = nil) {
        self.latestRun = latestRun
        self.credentials = credentials
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
        response.message.attachments = attachmentPaths.map { path in
            .with {
                $0.path = path
                $0.name = (path as NSString).lastPathComponent
            }
        }
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

// The slice B fixtures, in an extension to keep the struct body short.
extension PreviewClient {
    public func updateBot(_ request: Hearth_V1_UpdateBotRequest) async throws -> Hearth_V1_Bot {
        var bot = Fixtures.bot
        if !request.name.isEmpty { bot.name = request.name }
        if !request.role.isEmpty { bot.role = request.role }
        if !request.jobDescription.isEmpty { bot.jobDescription = request.jobDescription }
        if request.tier != .unspecified { bot.tier = request.tier }
        if !request.avatar.isEmpty { bot.avatar = request.avatar }
        if !request.modelOverride.isEmpty { bot.modelOverride = request.modelOverride }
        if request.clearModelOverride_p { bot.modelOverride = "" }
        return bot
    }

    public func listNotes(botID: String) async throws -> Hearth_V1_ListNotesResponse {
        .with {
            $0.notes = Fixtures.notes.filter { $0.botID == botID }
            $0.cap = Fixtures.noteCap
        }
    }

    public func editNote(id: String, text: String) async throws -> Hearth_V1_Note {
        var note = Fixtures.notes.first { $0.id == id } ?? Fixtures.notes[0]
        note.text = text
        return note
    }

    public func deleteNote(id: String) async throws {}

    public func pinNote(id: String, pinned: Bool) async throws -> Hearth_V1_Note {
        var note = Fixtures.notes.first { $0.id == id } ?? Fixtures.notes[0]
        note.pinned = pinned
        return note
    }

    public func listPages(botID: String) async throws -> Hearth_V1_ListPagesResponse {
        .with {
            $0.pages = Fixtures.handbookPages.filter {
                botID.isEmpty || $0.scope.everyone || $0.scope.botID == botID
            }
            $0.meter = Fixtures.handbookMeter
        }
    }

    public func putPage(
        _ request: Hearth_V1_PutPageRequest
    ) async throws -> Hearth_V1_PutPageResponse {
        // The core's refusal, in its words, so a store has one to show.
        if request.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            throw PageRefusal(message: "a page needs a title")
        }
        return .with {
            $0.page = .with {
                $0.id = request.id.isEmpty ? "page-new" : request.id
                $0.title = request.title
                $0.markdown = request.markdown
                $0.scope = request.scope
                $0.bytes = Int64(request.markdown.utf8.count)
                $0.updatedAt = Fixtures.time(30)
            }
            $0.meter = Fixtures.handbookMeter
        }
    }

    public func deletePage(id: String) async throws -> Hearth_V1_ContextMeter {
        Fixtures.handbookMeter
    }

    public func usageSummary(month: String) async throws -> Hearth_V1_SummaryResponse {
        Fixtures.usageSummary(month: month)
    }

    public func setBudgets(
        orgCents: Int64, bots: [Hearth_V1_BotBudget]
    ) async throws -> Hearth_V1_SummaryResponse {
        var summary = Fixtures.usageSummary(month: "")
        summary.orgBudgetCents = orgCents
        for budget in bots {
            guard let index = summary.bots.firstIndex(where: { $0.botID == budget.botID }) else {
                continue
            }
            summary.bots[index].budgetCents = budget.cents
        }
        return summary
    }

    public func resume(botID: String) async throws -> Hearth_V1_SummaryResponse {
        var summary = Fixtures.usageSummary(month: "")
        if botID.isEmpty {
            summary.orgPaused = false
        } else if let index = summary.bots.firstIndex(where: { $0.botID == botID }) {
            summary.bots[index].paused = false
        }
        return summary
    }

    public func tiers() async throws -> Hearth_V1_TiersResponse {
        .with {
            $0.tiers = Fixtures.tierChoices
            $0.options = Fixtures.modelOptions
        }
    }

    public func setTiers(_ tiers: [Hearth_V1_TierChoice]) async throws -> [Hearth_V1_TierChoice] {
        tiers
    }

    public func listServiceModels(
        _ request: Hearth_V1_ListServiceModelsRequest
    ) async throws -> [String] {
        Fixtures.ollamaModels
    }

    public func syncCredentials(
        _ records: [Hearth_V1_Credential]
    ) async throws -> [Hearth_V1_Credential] {
        credentials?.sync(records) ?? []
    }

    public func exportCredential(name: String) async throws -> Hearth_V1_Credential {
        credentials?.export(name) ?? .with { $0.name = name }
    }
}

/// A plain-words refusal, as the core's invalid_argument carries.
struct PageRefusal: LocalizedError {
    let message: String
    var errorDescription: String? { message }
}
