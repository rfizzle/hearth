// What SetupStore asks the core, by kind of script: the draft, the rules'
// check and the create. A project's go through ProjectRequest and
// ProjectsStore; a hire's through HireRequest straight to the client, since
// the team list hears of the new bot from the core's own event. The hire
// opener's chips live here too: the core's role templates, where a press puts
// the template's job description in the box, its role and suggested tier on
// the card, and remembers its id for the draft and the hire.
// See docs/model.md#people and docs/model.md#owner-interaction

import Foundation
import HearthKit
import HearthUI

extension SetupStore {
    /// Fetches the role templates the opener offers; a script without
    /// starters asks for none, and a failed fetch leaves the opener without
    /// chips, which are a help and not a gate.
    public func load() async {
        guard script.offersStarters, templates.isEmpty else { return }
        templates = (try? await client.listTemplates()) ?? []
    }

    /// Puts `template`'s job description in the box, and its role and
    /// suggested tier on the card, and remembers it for the draft and the hire.
    public func pick(_ template: Hearth_V1_RoleTemplate) {
        templateID = template.id
        paragraph = template.jobDescription
        var role = WidgetDraft()
        role.text = template.role
        drafts["role"] = role
        var tier = WidgetDraft()
        tier.choice = HireRequest.id(of: template.suggestedTier)
        drafts["tier"] = tier
    }

    /// Asks the core to draft the fields from `description`, and takes them.
    func requestDraft(_ description: String) async throws {
        switch script.kind {
        case .project:
            take(try await client.draftProject(description: description))
        case .hire:
            let request = HireRequest.draft(
                description: description, templateID: templateID, drafts)
            take(try await client.draftBot(request))
        }
    }

    /// Asks the core's rules about the fields as they stand.
    func check() async throws -> [Hearth_V1_Concern] {
        switch script.kind {
        case .project: try await client.checkProject(ProjectRequest.check(drafts))
        case .hire: try await client.checkBot(HireRequest.check(drafts))
        }
    }

    /// Takes the core's proposal for a bot into the fields it filled, with
    /// its concerns.
    func take(_ reply: Hearth_V1_DraftBotResponse) {
        concerns = []
        for (id, draft) in HireRequest.drafts(from: reply)
        where !draft.isEmpty(for: script.field(id)?.widget ?? .init()) {
            drafts[id] = draft
        }
        concerns = Array(reply.concerns.prefix(Self.maxConcerns))
    }

    /// Hires the bot the card shows; the core's refusal stays in `saveError`.
    func hire() async -> Outcome? {
        hireError = nil
        isBusy = true
        defer { isBusy = false }
        do {
            let bot = try await client.createBot(
                HireRequest.create(drafts, templateID: templateID))
            outcome = .bot(bot)
            return outcome
        } catch {
            hireError = error.localizedDescription
            return nil
        }
    }
}
