// HireStore holds the hire form: the templates the core offers, which
// starting point is picked, and the fields the owner edits before Create. A
// template prefills the role, job description and tier; the name and avatar
// are always the owner's. Whether the text is any good is the core's call; the
// store only keeps Create off while the name or the job description is blank.
//
// "Improve this" asks the core for a proposal and keeps it beside the owner's
// text, never in place of it: the owner's fields change only when they press
// "Use this", and "Keep mine" drops the proposal. A working-style chip appends
// one visible sentence to the proposal's job description, so what Create sends
// is always text the owner has seen on screen.
// See docs/model.md#people

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

    /// A working style the owner can give a proposal with one tap.
    public enum Style: String, CaseIterable, Sendable {
        case direct
        case warm
        case thorough
        case cautious

        /// The chip's word.
        public var title: String { rawValue.capitalized }

        /// The sentence a chosen chip adds to the job description.
        public var sentence: String {
            switch self {
            case .direct: "Be direct and brief, and lead with the answer."
            case .warm: "Be warm and friendly, and keep a personal tone."
            case .thorough: "Be thorough: check the details and explain how you got there."
            case .cautious: "Be cautious: ask before guessing or doing anything hard to undo."
            }
        }
    }

    /// What the core proposed, as fields the owner can still change.
    public struct Proposal: Equatable, Sendable {
        public var name: String
        public var role: String
        public var jobDescription: String
        public var workingStyle: String
        public var tier: Hearth_V1_Tier
        /// The chip whose sentence is at the end of `jobDescription`, if any.
        public fileprivate(set) var style: Style?

        /// What a view shows when no proposal exists; never sent anywhere.
        public static let empty = Proposal(
            name: "", role: "", jobDescription: "", workingStyle: "", tier: .balanced, style: nil)
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
    /// The core's proposal for the owner's text, shown beside it until the
    /// owner uses it or keeps their own.
    public var proposal: Proposal?
    /// Whether a proposal is being asked for.
    public private(set) var isDrafting = false
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

    /// Whether "Improve this" is on: there is text to improve and no request
    /// in flight.
    public var canDraft: Bool {
        !jobDescription.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty && !isDrafting
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
        proposal = nil
        role = template.role
        jobDescription = template.jobDescription
        tier = template.suggestedTier == .unspecified ? .balanced : template.suggestedTier
    }

    /// Starts from a blank form.
    public func describe() {
        start = .describe
        proposal = nil
        role = ""
        jobDescription = ""
        tier = .balanced
    }

    /// Asks the core to propose a bot from the job description as it stands.
    /// The owner's fields are untouched; the proposal is shown beside them.
    public func improve() async {
        guard canDraft else { return }
        isDrafting = true
        defer { isDrafting = false }
        do {
            let draft = try await client.draftBot(
                .with {
                    $0.description_p = jobDescription
                    if case .template(let id) = start { $0.templateID = id }
                    $0.tier = tier
                })
            proposal = Proposal(
                name: draft.name, role: draft.role, jobDescription: draft.jobDescription,
                workingStyle: draft.workingStyle,
                tier: draft.tier == .unspecified ? tier : draft.tier)
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Adds the chip's sentence to the proposal's job description, replacing
    /// the sentence of the chip chosen before; choosing the same chip again
    /// takes its sentence back out.
    public func choose(_ style: Style) {
        guard var draft = proposal else { return }
        if let old = draft.style {
            draft.jobDescription = Self.removing(old.sentence, from: draft.jobDescription)
        }
        if draft.style == style {
            draft.style = nil
        } else {
            let body = draft.jobDescription.trimmingCharacters(in: .whitespacesAndNewlines)
            draft.jobDescription = body.isEmpty ? style.sentence : body + " " + style.sentence
            draft.style = style
        }
        proposal = draft
    }

    /// Moves the proposal into the form: the role, the tier and the job
    /// description (with the working style as a closing line), and the name
    /// only while the owner has not typed one.
    public func useProposal() {
        guard let draft = proposal else { return }
        if name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { name = draft.name }
        role = draft.role
        tier = draft.tier
        let style = draft.workingStyle.trimmingCharacters(in: .whitespacesAndNewlines)
        let body = draft.jobDescription.trimmingCharacters(in: .whitespacesAndNewlines)
        jobDescription = style.isEmpty ? body : body + "\n\nWorking style: " + style
        proposal = nil
    }

    /// Drops the proposal and leaves the owner's text as it was.
    public func keepMine() {
        proposal = nil
    }

    private static func removing(_ sentence: String, from text: String) -> String {
        text.replacingOccurrences(of: " " + sentence, with: "")
            .replacingOccurrences(of: sentence, with: "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
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
                    if case .template(let id) = start { $0.templateID = id }
                })
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }
}
