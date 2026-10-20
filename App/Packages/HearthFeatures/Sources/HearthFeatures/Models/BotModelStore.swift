// BotModelStore is the Advanced choice on the Access sheet: the model pinned
// for one bot, which wins over the tier's, and the bot's job description as
// the core keeps it, shown and never edited here. The owner edits the pin; Save
// sends it only when it changed, as the pin or as the flag that clears it. The
// core checks it and answers with the bot, whose pin is then what is shown.
// See docs/model.md#providers-and-models

import Foundation
import HearthKit
import HearthUI
import Observation

/// The store behind the Access sheet's Advanced disclosure.
@MainActor
@Observable
public final class BotModelStore {
    /// The bot the pin belongs to.
    public let botID: String
    /// The job description as the core keeps it.
    public let jobDescription: String
    /// The pinned model as the owner has it now; empty uses the tier's.
    public var pinned: String
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// Whether a Save is waiting on the core.
    public private(set) var isSaving = false
    /// Whether the last Save went through, or had nothing to send.
    public private(set) var didSave = false

    private var kept: String
    private let client: any HearthClient
    private let models: ModelsStore

    /// A store for `bot` over `client`, offering the models `models` lists.
    public init(client: any HearthClient, models: ModelsStore, bot: Hearth_V1_Bot) {
        self.client = client
        self.models = models
        botID = bot.id
        jobDescription = bot.jobDescription
        pinned = bot.modelOverride
        kept = bot.modelOverride
    }

    /// Fetches the models the owner may pick.
    public func load() async {
        await models.load()
    }

    /// Whether the owner changed the pin since the core last said.
    public var hasChanged: Bool { pinned != kept }

    /// The models to offer, with the bot's own pin among them even when the
    /// accounts no longer offer it, so the choice never reads as "no pin".
    public var options: [Hearth_V1_ModelOption] {
        let offered = models.options
        guard let current = Hearth_V1_ModelOption(pinned: kept),
            !offered.contains(where: { $0.pinned == kept })
        else { return offered }
        return offered + [current]
    }

    /// The update to send: the pin, or the flag that clears it.
    var request: Hearth_V1_UpdateBotRequest {
        .with {
            $0.id = botID
            if pinned.isEmpty {
                $0.clearModelOverride_p = true
            } else {
                $0.modelOverride = pinned
            }
        }
    }

    /// Sends the pin when it changed; otherwise there is nothing to send.
    public func save() async {
        didSave = false
        guard hasChanged else {
            didSave = true
            return
        }
        isSaving = true
        defer { isSaving = false }
        do {
            let bot = try await client.updateBot(request)
            kept = bot.modelOverride
            pinned = kept
            error = nil
            didSave = true
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Puts back what the core last said.
    public func discard() {
        pinned = kept
        error = nil
    }
}
