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
    /// Asks the model to propose a name, role, job description and working
    /// style from the owner's words. It only proposes; nothing is stored.
    func draftBot(_ request: Hearth_V1_DraftBotRequest) async throws -> Hearth_V1_DraftBotResponse
    /// What looks weak about a hire, by rule and with no model: at most three
    /// concerns, none blocking. Nothing is stored.
    func checkBot(_ request: Hearth_V1_CheckBotRequest) async throws -> [Hearth_V1_Concern]

    /// The conversations `botID` is in; an empty id lists every conversation.
    func listConversations(botID: String) async throws -> [Hearth_V1_Conversation]
    /// One page of messages below `beforeSeq` (0 for the newest), ascending by
    /// seq; a `limit` of 0 asks for the core's default page.
    func listMessages(conversationID: String, beforeSeq: Int64, limit: Int32) async throws
        -> Hearth_V1_ListMessagesResponse
    /// Stores the owner's message and starts one run; the reply arrives on
    /// `subscribe`. `attachmentPaths` are absolute paths the app resolved; the
    /// core copies each into the Inbox unless the bot may already read it.
    func send(conversationID: String, text: String, attachmentPaths: [String]) async throws
        -> Hearth_V1_SendResponse
    /// Stops a run, keeping what the bot had written so far.
    func cancel(runID: String) async throws
    /// One conversation with its latest run, so a screen opened mid-run shows it.
    func getConversation(id: String) async throws -> Hearth_V1_Conversation
    /// Runs the owner message behind a failed or cancelled run again, as a new
    /// run; refused while a run in the conversation is working or waiting.
    func retry(runID: String) async throws -> Hearth_V1_Run

    /// What the bot may do, one row per capability and per folder.
    func listGrants(botID: String) async throws -> [Hearth_V1_Grant]
    /// Replaces the bot's grants whole; the core answers with what it kept.
    func setGrants(botID: String, _ grants: [Hearth_V1_Grant]) async throws -> [Hearth_V1_Grant]

    /// What waits on the owner; closed items only when asked for.
    func listInbox(includeClosed: Bool) async throws -> [Hearth_V1_InboxItem]
    /// Answers an approval with one of the four decisions; the run resumes.
    func decide(itemID: String, _ decision: Hearth_V1_Decision, reason: String) async throws
        -> Hearth_V1_DecideResponse
    /// Replies to a question, in words or through its control; the run resumes.
    func answer(itemID: String, _ text: String, value: Hearth_V1_WidgetAnswer?) async throws
        -> Hearth_V1_AnswerResponse
    /// Closes an item that needs no decision, such as a failure.
    func acknowledge(itemID: String) async throws -> Hearth_V1_InboxItem

    /// Every provider and whether it has a credential; never the credential.
    func listAccounts() async throws -> [Hearth_V1_ProviderAccount]
    /// Stores an API key for `provider`.
    func setAPIKey(provider: Hearth_V1_Provider, key: String) async throws
        -> Hearth_V1_ProviderAccount
    /// Forgets the credential for `provider`.
    func clearCredential(provider: Hearth_V1_Provider) async throws
    /// Adds an OpenAI-compatible service, or changes the one `request.profile`
    /// names, from its name, address, key and default model; the account it
    /// returns is the service's own card.
    func saveService(_ request: Hearth_V1_SetAPIKeyRequest) async throws
        -> Hearth_V1_ProviderAccount
    /// Removes the added service `profile` and its key.
    func removeService(profile: String) async throws
    /// The services the core knows the address of, offered when the owner
    /// adds another service.
    func listKnownServices() async throws -> [Hearth_V1_KnownService]
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

    /// Changes what the owner may edit on a bot; empty strings keep the stored
    /// value and `clearModelOverride` drops the Advanced override.
    func updateBot(_ request: Hearth_V1_UpdateBotRequest) async throws -> Hearth_V1_Bot
    /// What the bot remembers, pinned notes first, and how many it may keep.
    func listNotes(botID: String) async throws -> Hearth_V1_ListNotesResponse
    /// Replaces the words of a note.
    func editNote(id: String, text: String) async throws -> Hearth_V1_Note
    /// Makes the bot forget a note.
    func deleteNote(id: String) async throws
    /// Pins a note so the bot never tidies it away, or lets it go.
    func pinNote(id: String, pinned: Bool) async throws -> Hearth_V1_Note

    /// The handbook pages `botID` reads, or every page for an empty id, and
    /// how much of the context budget they take.
    func listPages(botID: String) async throws -> Hearth_V1_ListPagesResponse
    /// Adds a page, or replaces the page `request.id` names.
    func putPage(_ request: Hearth_V1_PutPageRequest) async throws -> Hearth_V1_PutPageResponse
    /// Removes a page; the meter is what remains.
    func deletePage(id: String) async throws -> Hearth_V1_ContextMeter

    /// What the bots cost in `month` ("2026-10"; empty is this month).
    func usageSummary(month: String) async throws -> Hearth_V1_SummaryResponse
    /// Sets the org's monthly budget and each listed bot's, in cents; 0 is none.
    func setBudgets(orgCents: Int64, bots: [Hearth_V1_BotBudget]) async throws
        -> Hearth_V1_SummaryResponse
    /// Lets a paused bot, or the whole org for an empty id, work again this month.
    func resume(botID: String) async throws -> Hearth_V1_SummaryResponse

    /// The model each tier asks for, and every model the owner may pick.
    func tiers() async throws -> Hearth_V1_TiersResponse
    /// Maps tiers to models; the core answers with the mapping it kept.
    func setTiers(_ tiers: [Hearth_V1_TierChoice]) async throws -> [Hearth_V1_TierChoice]
    /// The model ids a service serves: by `baseURL` and `key` before a key is
    /// saved, or by `profile` for a saved service.
    func listServiceModels(_ request: Hearth_V1_ListServiceModelsRequest) async throws -> [String]
    /// Hands the core the records the app keeps in the Keychain; the reply is
    /// the records the core still held in files, for the app to store.
    func syncCredentials(_ records: [Hearth_V1_Credential]) async throws -> [Hearth_V1_Credential]
    /// One record to store after a credential change.
    func exportCredential(name: String) async throws -> Hearth_V1_Credential

    /// Every project that is proposed, active or done; archived too when asked.
    func listProjects(includeArchived: Bool) async throws -> [Hearth_V1_Project]
    /// Asks the model to turn a paragraph into a project's name, brief and
    /// target date, with at most three concerns. It only proposes; nothing is
    /// stored. Unavailable while no model is set up.
    func draftProject(description: String) async throws -> Hearth_V1_DraftProjectResponse
    /// What looks weak about a project's fields, by rule and with no model.
    func checkProject(_ request: Hearth_V1_CheckProjectRequest) async throws -> [Hearth_V1_Concern]
    /// Creates an active project, or confirms the proposal `request.proposalID` names.
    func createProject(_ request: Hearth_V1_CreateProjectRequest) async throws -> Hearth_V1_Project
    /// Replaces the name, brief and target date.
    func updateProject(_ request: Hearth_V1_UpdateProjectRequest) async throws -> Hearth_V1_Project
    /// The owner's mark.
    func markProjectDone(id: String) async throws -> Hearth_V1_Project
    /// Keeps the folder and the conversation; "Not now" on a proposal.
    func archiveProject(id: String) async throws -> Hearth_V1_Project

    /// Every measure id that resolves now, with its label and no value.
    func listMeasures() async throws -> [Hearth_V1_MeasureValue]
    /// One value per resolved id, in the order asked, at most 100. A subject
    /// that is gone is a value with `missing` set, not an error.
    func readMeasures(ids: [String]) async throws -> [Hearth_V1_MeasureValue]
    /// The five shipped templates, the owner's after them, and each file that
    /// could not be used with its problem.
    func listDeliverableTemplates() async throws -> [Hearth_V1_DeliverableTemplate]
    /// Validates a template document and keeps it in the Templates folder.
    func saveDeliverableTemplate(json: String) async throws -> Hearth_V1_DeliverableTemplate
    /// Removes one of the owner's templates.
    func deleteDeliverableTemplate(id: String) async throws

    /// Every change the core makes after `sequence` (0 for live events only),
    /// in order. The stream finishes with the error that ended it.
    func subscribe(after sequence: UInt64) -> AsyncThrowingStream<
        Hearth_V1_SubscribeResponse, Error
    >
}

extension HearthClient {
    /// Sends the owner's words with nothing attached.
    public func send(conversationID: String, text: String) async throws -> Hearth_V1_SendResponse {
        try await send(conversationID: conversationID, text: text, attachmentPaths: [])
    }

    /// Replies to a question in prose.
    public func answer(itemID: String, _ text: String) async throws -> Hearth_V1_AnswerResponse {
        try await answer(itemID: itemID, text, value: nil)
    }
}

/// A reply from the core the client cannot use.
public enum HearthClientError: Error, Equatable {
    /// The sign-in page the core returned is not a URL.
    case invalidAuthorizeURL(String)
}
