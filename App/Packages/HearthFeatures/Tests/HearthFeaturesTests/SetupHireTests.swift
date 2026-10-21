// The hire script's store over PreviewClient: a role chip fills the box and
// its template id reaches the draft and the hire, a blank start sends none,
// the draft fills the card with its concern and closes the job description on
// the working style, a hire sends what the card shows, no model asks the name
// and the role then checks the rules, an unavailable draft falls back to the
// job description, the form is remembered under the hire's own key, and the
// script's ids, order and examples hold.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

/// A store for `script` with defaults of its own.
@MainActor
private func makeStore(
    _ script: SetupScript = .hire, client: PreviewClient = PreviewClient(),
    canDraft: Bool = true, defaults: UserDefaults = freshHireDefaults()
) -> SetupStore {
    SetupStore(
        script: script, client: client, projects: makeSidebar(client: client).projects,
        canDraft: canDraft, defaults: defaults)
}

private func freshHireDefaults() -> UserDefaults {
    UserDefaults(suiteName: "setup-hire-tests-\(UUID().uuidString)") ?? .standard
}

private let paragraph = "Someone to keep track of our invoices."

@MainActor
@Test func aRoleChipFillsTheBoxAndRemembersItsTemplate() async {
    let store = makeStore()
    #expect(store.templates.isEmpty)
    await store.load()
    #expect(store.templates.count == Fixtures.templates.count)
    let writer = Fixtures.templates[1]
    store.pick(writer)
    #expect(store.paragraph == writer.jobDescription)
    #expect(store.templateID == writer.id)
    #expect(store.drafts["role"]?.text == writer.role)
    #expect(store.drafts["tier"]?.choice == "best")
    #expect(store.canContinue)
}

@MainActor
@Test func theProjectScriptOffersNoChips() async {
    let store = makeStore(.project)
    await store.load()
    #expect(store.templates.isEmpty)
}

@MainActor
@Test func theTemplateIdTravelsWithTheDraftAndTheHire() async throws {
    let client = PreviewClient()
    let store = makeStore(client: client)
    await store.load()
    store.pick(Fixtures.templates[0])
    await store.primary()
    let draft = try #require(client.botRequests.lastDraft)
    #expect(draft.templateID == Fixtures.templates[0].id)
    #expect(draft.description_p == Fixtures.templates[0].jobDescription)
    #expect(draft.tier == .balanced)
    #expect(store.step == .card)
    await store.primary()
    let create = try #require(client.botRequests.lastCreate)
    #expect(create.templateID == Fixtures.templates[0].id)
    guard case .bot = try #require(store.outcome) else { return }
}

@MainActor
@Test func aBlankStartSendsNoTemplate() async throws {
    let client = PreviewClient()
    let store = makeStore(client: client)
    store.paragraph = paragraph
    await store.primary()
    #expect(try #require(client.botRequests.lastDraft).templateID.isEmpty)
    await store.primary()
    #expect(try #require(client.botRequests.lastCreate).templateID.isEmpty)
}

@MainActor
@Test func theHireDraftFillsTheCardWithItsConcernAndClosesOnTheStyle() async {
    let store = makeStore()
    store.paragraph = paragraph
    await store.primary()
    #expect(store.step == .card)
    #expect(store.primaryLabel == "Hire")
    #expect(store.drafts["name"]?.text == Fixtures.draft.name)
    #expect(store.drafts["role"]?.text == Fixtures.draft.role)
    let shown = Fixtures.draft.jobDescription + "\n\nWorking style: " + Fixtures.draft.workingStyle
    #expect(store.drafts["job_description"]?.text == shown)
    #expect(store.drafts["tier"]?.choice == "balanced")
    #expect(store.concerns(for: "job_description").count == 1)
    store.keep(store.concerns[0])
    #expect(store.concerns.isEmpty)
    #expect(store.canContinue)
}

@MainActor
@Test func hireSendsWhatTheCardShows() async throws {
    let store = makeStore()
    store.paragraph = paragraph
    await store.primary()
    store.drafts["name", default: WidgetDraft()].text = "  Sam  "
    store.drafts["role", default: WidgetDraft()].text = "Plant Lead"
    store.drafts["job_description", default: WidgetDraft()].text = "You water the plants."
    store.drafts["tier", default: WidgetDraft()].choice = "fast"
    store.drafts["avatar", default: WidgetDraft()].text = "\u{1F331}"
    await store.primary()
    guard case .bot(let bot) = try #require(store.outcome) else { return }
    #expect(bot.name == "Sam")
    #expect(bot.role == "Plant Lead")
    #expect(bot.jobDescription == "You water the plants.")
    #expect(bot.tier == .fast)
    #expect(bot.avatar == "\u{1F331}")
    #expect(store.saveError == nil)
}

@MainActor
@Test func withoutAModelAHireAsksNameAndRoleThenChecksTheRules() async throws {
    let client = PreviewClient()
    let store = makeStore(client: client, canDraft: false)
    #expect(store.primaryLabel == "Next")
    store.paragraph = "Keeps the books."
    await store.primary()
    #expect(store.drafts["job_description"]?.text == "Keeps the books.")
    #expect(store.step == .asking("name"))
    store.drafts["name", default: WidgetDraft()].text = "Sam"
    await store.primary()
    #expect(store.step == .asking("role"))
    store.drafts["role", default: WidgetDraft()].text = "Bookkeeper"
    await store.primary()
    #expect(store.step == .card)
    #expect(store.concerns.map(\.field) == ["job_description"])
    #expect(try #require(client.botRequests.lastCheck).role == "Bookkeeper")
    #expect(client.botRequests.lastDraft == nil)
}

@MainActor
@Test func anUnavailableHireDraftFallsBackToTheJobDescription() async {
    let store = makeStore(client: PreviewClient(modelUnavailable: true))
    store.paragraph = paragraph
    await store.primary()
    #expect(store.isFallback)
    #expect(store.drafts["job_description"]?.text == paragraph)
    #expect(store.step == .asking("name"))
}

@MainActor
@Test func theHireFormIsRememberedUnderItsOwnKey() async {
    let defaults = freshHireDefaults()
    let store = makeStore(defaults: defaults)
    store.showForm()
    #expect(defaults.bool(forKey: "setup.hire.form"))
    #expect(!defaults.bool(forKey: "setup.project.form"))
    #expect(makeStore(defaults: defaults).step == .form)
    await store.showGuide()
    #expect(store.step == .opener)
}

@Test func theHireScriptHoldsTogether() {
    let script = SetupScript.hire
    let fields = script.fields
    #expect(fields.map(\.id) == ["name", "role", "job_description", "tier", "avatar"])
    let firstOptional = fields.firstIndex { !$0.required } ?? fields.count
    #expect(fields[firstOptional...].allSatisfy { !$0.required })
    #expect(fields.contains { $0.id == script.opener.seeds })
    #expect(script.offersStarters)
    #expect(script.preferenceKey == "setup.hire.form")
    for field in fields {
        var draft = WidgetDraft()
        if case .singleChoice = field.widget.kind {
            draft.choice = field.example
        } else {
            draft.text = field.example
        }
        #expect(draft.isValid(for: field.widget), "\(field.id)")
        #expect(!draft.isEmpty(for: field.widget), "\(field.id)")
    }
}
