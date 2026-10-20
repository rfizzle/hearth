// The last bot requests a PreviewClient was given, kept so a store test can
// say what a sheet sent: the draft's, the check's and the hire's, template id
// and all.

import Foundation

/// The bot requests PreviewClient last received, shared by its copies.
public final class PreviewBotRequests: @unchecked Sendable {
    private let lock = NSLock()
    private var draft: Hearth_V1_DraftBotRequest?
    private var create: Hearth_V1_CreateBotRequest?
    private var check: Hearth_V1_CheckBotRequest?

    /// A record of nothing sent yet.
    public init() {}

    /// The latest `draftBot` request, if there was one.
    public var lastDraft: Hearth_V1_DraftBotRequest? { lock.withLock { draft } }
    /// The latest `createBot` request, if there was one.
    public var lastCreate: Hearth_V1_CreateBotRequest? { lock.withLock { create } }
    /// The latest `checkBot` request, if there was one.
    public var lastCheck: Hearth_V1_CheckBotRequest? { lock.withLock { check } }

    func record(_ request: Hearth_V1_DraftBotRequest) { lock.withLock { draft = request } }
    func record(_ request: Hearth_V1_CreateBotRequest) { lock.withLock { create = request } }
    func record(_ request: Hearth_V1_CheckBotRequest) { lock.withLock { check = request } }
}

extension PreviewClient {
    public func createBot(_ request: Hearth_V1_CreateBotRequest) async throws -> Hearth_V1_Bot {
        botRequests.record(request)
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
        botRequests.record(request)
        guard !modelUnavailable else {
            throw HearthClientError.unavailable("no model is set up; sign in to one in Settings")
        }
        return Fixtures.draft
    }
}
