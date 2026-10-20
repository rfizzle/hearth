// SetupStore walks one setup sheet through its script: the opener, where the
// owner says in a paragraph what they want; the draft, which the core makes
// from that paragraph with the Fast tier; one asking step for each required
// field the draft left empty; and the card, where every field is the owner's
// to change before Create. With no model to draft with (none signed in, or
// the core's `unavailable` from the draft) the sheet carries on without one:
// the paragraph fills the script's seed field, every other required field is
// asked in turn, the card asks the core's rules for their concerns, and a
// quiet line says why in the core's words. The core's concerns are at most
// three, each about one field; using one writes its suggestion into the
// field, keeping one or editing the field puts it away; none of them blocks
// Create. "Just show me the form" swaps to the grouped form and is remembered
// in the defaults under the script's key, the one `@AppStorage` reads; editing
// a project always opens the form. The store decides nothing about the
// project: the draft, the concerns and the refusal are the core's, and the
// one check made here is the sheet's, that each required field holds
// something within its control's limits.
// See docs/model.md#owner-interaction and docs/design.md#forms-sheets-and-settings

import Foundation
import HearthKit
import HearthUI
import Observation

/// The store behind a setup sheet.
@MainActor
@Observable
public final class SetupStore {
    /// The one step on screen.
    public enum Step: Equatable, Sendable {
        /// The opening question and its box.
        case opener
        /// The question for the field with this id.
        case asking(String)
        /// Every field, filled in, to change before Create.
        case card
        /// The grouped form, by preference or to edit.
        case form
    }

    /// What the sheet made.
    public enum Outcome: Equatable, Sendable {
        case project(Hearth_V1_Project)
    }

    /// The script the sheet follows.
    public let script: SetupScript
    /// The project being edited, if the sheet edits one.
    public let editing: Hearth_V1_Project?
    /// The step on screen.
    public private(set) var step: Step
    /// The owner's answer to the opener, as typed.
    public var paragraph = ""
    /// Each field's value by field id, as the card and the steps show it.
    public var drafts: [String: WidgetDraft] = [:] {
        didSet { putAwayEdited(from: oldValue) }
    }
    /// The core's concerns still waiting on the owner, at most three.
    public private(set) var concerns: [Hearth_V1_Concern] = []
    /// Whether a model may be signed in; false starts the sheet without one.
    public var canDraft: Bool
    /// The core's words for why it could not draft, once it has said so.
    public private(set) var draftFailure: String?
    /// A failed draft's words other than `unavailable`, shown under the box.
    public private(set) var error: String?
    /// Whether a draft or a check is in flight.
    public private(set) var isBusy = false
    /// What Create or Save made, once it has.
    public private(set) var outcome: Outcome?

    /// The fields asked after the opener, in order.
    private var asked: [String] = []
    /// Whether the card has asked the core's rules once.
    private var checked = false
    private let client: any HearthClient
    private let projects: ProjectsStore
    @ObservationIgnored private let defaults: UserDefaults
    @ObservationIgnored private let preferenceKey: String

    /// A store for `script` over `client`, saving through `projects`;
    /// `editing` opens the form on that project. `preferenceKey` and
    /// `defaults` are where "Just show me the form" is remembered; tests and
    /// the gallery pass their own.
    public init(
        script: SetupScript, client: any HearthClient, projects: ProjectsStore,
        canDraft: Bool = true, editing: Hearth_V1_Project? = nil,
        preferenceKey: String? = nil, defaults: UserDefaults = .standard
    ) {
        self.script = script
        self.client = client
        self.projects = projects
        self.canDraft = canDraft
        self.editing = editing
        self.defaults = defaults
        let key = preferenceKey ?? script.preferenceKey
        self.preferenceKey = key
        if let editing {
            drafts = ProjectRequest.drafts(
                name: editing.name, brief: editing.brief, targetDate: editing.targetDate)
            step = .form
        } else {
            step = defaults.bool(forKey: key) ? .form : .opener
        }
    }

    /// Whether the sheet goes on without a model.
    public var isFallback: Bool {
        !canDraft || draftFailure != nil
    }

    /// The quiet line under the opener that says why there is no draft.
    public var fallbackLine: String? {
        if let draftFailure {
            return "I couldn\u{2019}t draft this (\(draftFailure)), so I\u{2019}ll ask you a few "
                + "things one at a time."
        }
        return canDraft
            ? nil
            : "No one is signed in to a model yet, so I\u{2019}ll ask you a few things one at a time."
    }

    /// Where the owner is, in words: "One more thing" on the last question,
    /// "Last step" on the card; nil on the opener and the form, where it is
    /// not yet known or does not apply.
    public var progress: String? {
        switch step {
        case .asking(let id):
            let left = asked.count - 1 - (asked.firstIndex(of: id) ?? asked.count - 1)
            return left == 0 ? "One more thing" : "A few more things"
        case .card: return "Last step"
        case .opener, .form: return nil
        }
    }

    /// The primary button's label on this step.
    public var primaryLabel: String {
        switch step {
        case .opener: isFallback ? "Next" : "Draft"
        case .asking: "Next"
        case .card: script.createLabel
        case .form: editing == nil ? script.createLabel : "Save"
        }
    }

    /// Whether Back shows: from the second step on, never on the form.
    public var canGoBack: Bool {
        switch step {
        case .asking, .card: true
        case .opener, .form: false
        }
    }

    /// Whether a call is in flight, so the primary waits.
    public var isWorking: Bool {
        isBusy || projects.isSaving
    }

    /// The core's refusal of Create or Save, in its words.
    public var saveError: String? {
        projects.saveError
    }

    /// Whether the primary button is on: the opener has words, the field
    /// asked holds an answer, every required field on the card or the form
    /// does. A concern never turns it off.
    public var canContinue: Bool {
        guard !isWorking else { return false }
        switch step {
        case .opener: return !ProjectRequest.isBlank(paragraph)
        case .asking(let id): return script.field(id).map(isAnswered) ?? false
        case .card, .form: return script.fields.allSatisfy(isAnswered)
        }
    }

    /// The concerns about the field with `id`.
    public func concerns(for id: String) -> [Hearth_V1_Concern] {
        concerns.filter { $0.field == id }
    }

    /// The concerns about no field of the script, shown at the card's end.
    public var looseConcerns: [Hearth_V1_Concern] {
        concerns.filter { script.field($0.field) == nil }
    }

    /// Whether `field` holds an answer within its limits, or is optional and
    /// empty.
    func isAnswered(_ field: SetupScript.Field) -> Bool {
        let draft = drafts[field.id] ?? WidgetDraft()
        return draft.isEmpty(for: field.widget) ? !field.required : draft.isValid(for: field.widget)
    }

    /// The primary button: Draft or Next on the opener, Next on a question,
    /// Create or Save on the card and the form.
    public func primary() async {
        guard canContinue else { return }
        switch step {
        case .opener: await draft()
        case .asking: await next()
        case .card, .form: await create()
        }
    }

    /// Drafts the fields from the paragraph, or without a model fills the
    /// seed field with it, then asks what is still missing.
    public func draft() async {
        guard !ProjectRequest.isBlank(paragraph) else { return }
        error = nil
        if isFallback {
            seed()
        } else {
            isBusy = true
            do {
                let description = ProjectRequest.trimmed(paragraph)
                let reply = try await client.draftProject(description: description)
                isBusy = false
                take(reply)
            } catch let failure where HearthClientError.isUnavailable(failure) {
                isBusy = false
                draftFailure = failure.localizedDescription
                seed()
            } catch {
                isBusy = false
                self.error = error.localizedDescription
                return
            }
        }
        // The owner may have swapped to the form while the core drafted.
        guard step == .opener else { return }
        await ask()
    }

    /// Asks each required field still empty, in card order, then the card.
    func ask() async {
        asked = script.fields.filter { $0.required && !isAnswered($0) }.map(\.id)
        if let first = asked.first {
            step = .asking(first)
        } else {
            await showCard()
        }
    }

    /// Moves from a question to the next one, or to the card after the last.
    public func next() async {
        guard case .asking(let id) = step, let index = asked.firstIndex(of: id) else { return }
        if index + 1 < asked.count {
            step = .asking(asked[index + 1])
        } else {
            await showCard()
        }
    }

    /// Goes back a step, keeping everything typed.
    public func back() {
        switch step {
        case .asking(let id):
            let index = asked.firstIndex(of: id) ?? 0
            step = index > 0 ? .asking(asked[index - 1]) : .opener
        case .card:
            step = asked.last.map(Step.asking) ?? .opener
        case .opener, .form:
            break
        }
    }

    /// Writes `concern`'s suggestion into its field and puts it away.
    public func use(_ concern: Hearth_V1_Concern) {
        guard !concern.suggestion.isEmpty else { return keep(concern) }
        drafts[concern.field, default: WidgetDraft()].text = concern.suggestion
        keep(concern)
    }

    /// Puts `concern` away, leaving its field as the owner had it.
    public func keep(_ concern: Hearth_V1_Concern) {
        if let index = concerns.firstIndex(of: concern) { concerns.remove(at: index) }
    }

    /// Swaps to the form, keeping everything typed, and remembers the choice.
    public func showForm() {
        defaults.set(true, forKey: preferenceKey)
        step = .form
    }

    /// Swaps back to the guided steps and forgets the choice: to the card
    /// when something is typed, else to the opener.
    public func showGuide() async {
        defaults.removeObject(forKey: preferenceKey)
        asked = []
        let typed = script.fields.contains {
            !(drafts[$0.id] ?? WidgetDraft()).isEmpty(for: $0.widget)
        }
        if typed { await showCard() } else { step = .opener }
    }

    /// Creates the project the card shows, or saves the one being edited;
    /// the core's refusal stays in `saveError`.
    @discardableResult
    public func create() async -> Outcome? {
        guard script.fields.allSatisfy(isAnswered) else { return nil }
        let saved =
            if let editing {
                await projects.update(ProjectRequest.update(id: editing.id, drafts))
            } else {
                await projects.create(ProjectRequest.create(drafts))
            }
        outcome = saved.map(Outcome.project)
        return outcome
    }

    /// Takes the core's draft into the fields it filled, with its concerns.
    func take(_ reply: Hearth_V1_DraftProjectResponse) {
        concerns = []
        let drafted = ProjectRequest.drafts(
            name: reply.name, brief: reply.brief, targetDate: reply.targetDate)
        for (id, draft) in drafted where !draft.isEmpty(for: script.field(id)?.widget ?? .init()) {
            drafts[id] = draft
        }
        concerns = Array(reply.concerns.prefix(Self.maxConcerns))
    }

    /// The most concerns a card shows, the core's own cap.
    static let maxConcerns = 3

    /// Puts the paragraph in the seed field.
    private func seed() {
        drafts[script.opener.seeds, default: WidgetDraft()].text = ProjectRequest.trimmed(paragraph)
    }

    /// Shows the card; without a model, asks the core's rules once.
    private func showCard() async {
        step = .card
        guard isFallback, !checked else { return }
        checked = true
        isBusy = true
        defer { isBusy = false }
        // A failed check leaves the card as it is: concerns are a help, not a gate.
        if let found = try? await client.checkProject(ProjectRequest.check(drafts)), step == .card {
            concerns = Array(found.prefix(Self.maxConcerns))
        }
    }

    /// Puts away the concerns about a field the owner has changed.
    private func putAwayEdited(from old: [String: WidgetDraft]) {
        concerns.removeAll {
            (old[$0.field] ?? WidgetDraft()) != (drafts[$0.field] ?? WidgetDraft())
        }
    }
}
