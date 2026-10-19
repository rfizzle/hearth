// HireStore holds the hire form: the templates the core offers, which
// starting point is picked, and the fields the owner edits before Create. A
// template prefills the role, job description and tier; the name and avatar
// are always the owner's. Whether the text is any good is the core's call; the
// store only keeps Create off while the name or the job description is blank.

import Foundation
import HearthKit
import Observation

/// The store behind the hire sheet.
@MainActor
@Observable
public final class HireStore {
    /// Where the form started.
    public enum Start: Equatable, Sendable {
        /// A shipped template, by id.
        case template(String)
        /// A blank form, for the owner to describe the job.
        case describe
    }

    /// The templates the core offers.
    public private(set) var templates: [Hearth_V1_RoleTemplate] = []
    /// The starting point picked, if one has been.
    public private(set) var start: Start?
    /// A bot's name, as typed. See docs/model.md#people
    public var name = ""
    /// The bot's role, prefilled from the template and editable.
    public var role = ""
    /// What the bot does, prefilled from the template and editable.
    public var jobDescription = ""
    /// The quality tier; the template's suggestion, else Balanced.
    public var tier = Hearth_V1_Tier.balanced
    /// An emoji or an SF Symbol name; empty shows the name's initials.
    public var avatar = ""
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// The bot the core created, once Create has succeeded.
    public private(set) var createdBot: Hearth_V1_Bot?

    private let client: any HearthClient

    /// A store over `client`.
    public init(client: any HearthClient) {
        self.client = client
    }

    /// Whether Create is enabled: a name and a job description have been typed.
    public var canCreate: Bool {
        !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
            && !jobDescription.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// Fetches the templates from the core.
    public func load() async {
        do {
            templates = try await client.listTemplates()
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Starts from `template`, prefilling the role, job description and tier.
    public func pick(_ template: Hearth_V1_RoleTemplate) {
        start = .template(template.id)
        role = template.role
        jobDescription = template.jobDescription
        tier = template.suggestedTier == .unspecified ? .balanced : template.suggestedTier
    }

    /// Starts from a blank form.
    public func describe() {
        start = .describe
        role = ""
        jobDescription = ""
        tier = .balanced
    }

    /// Asks the core to create the bot from the fields as typed.
    public func create() async {
        guard canCreate else { return }
        do {
            createdBot = try await client.createBot(
                .with {
                    $0.name = name
                    $0.role = role
                    $0.jobDescription = jobDescription
                    $0.tier = tier
                    $0.avatar = avatar
                })
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }
}
