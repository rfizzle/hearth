// The guided setup sheet's store over PreviewClient: a full draft goes
// straight to the card, a draft missing a required field asks it next, a
// concern leaves when it is used, kept or its field is edited, the core's
// unavailable puts the paragraph in the goal and asks the rest, no model
// starts without one, the form preference is remembered and forgotten, Back
// keeps an answer, Create sends what the card shows, editing opens the form,
// and the project script's ids, order and examples hold.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

/// A store for the project script with defaults of its own.
@MainActor
private func makeSetup(
    client: PreviewClient = PreviewClient(), canDraft: Bool = true,
    editing: Hearth_V1_Project? = nil, defaults: UserDefaults = freshDefaults()
) -> SetupStore {
    SetupStore(
        script: .project, client: client,
        projects: makeSidebar(client: client).projects, canDraft: canDraft, editing: editing,
        defaults: defaults)
}

private func freshDefaults() -> UserDefaults {
    UserDefaults(suiteName: "setup-tests-\(UUID().uuidString)") ?? .standard
}

private let paragraph = "I want to start a studio newsletter."

@MainActor
@Test func aFullDraftGoesStraightToTheCardWithItsConcerns() async {
    let store = makeSetup()
    #expect(store.step == .opener)
    #expect(store.primaryLabel == "Draft")
    #expect(!store.canContinue)
    store.paragraph = paragraph
    await store.primary()
    #expect(store.step == .card)
    #expect(store.progress == "Last step")
    #expect(store.primaryLabel == "Create")
    #expect(store.drafts["name"]?.text == "Launch the newsletter")
    #expect(store.drafts["done_when"]?.text == "The first issue has gone out.")
    #expect(store.drafts["target_date"]?.date != nil)
    #expect(store.concerns(for: "done_when").count == 1)
    #expect(store.concerns(for: "name").count == 1)
    #expect(store.fallbackLine == nil)
    #expect(store.canContinue)
}

@MainActor
@Test func aDraftMissingDoneWhenAsksItNext() async {
    let store = makeSetup()
    var reply = Fixtures.projectDraft
    reply.brief.doneWhen = ""
    store.take(reply)
    await store.ask()
    #expect(store.step == .asking("done_when"))
    #expect(store.progress == "One more thing")
    #expect(store.canGoBack)
    #expect(!store.canContinue)
    store.drafts["done_when", default: WidgetDraft()].text = "Every subscriber has the issue."
    #expect(store.canContinue)
    await store.next()
    #expect(store.step == .card)
}

@MainActor
@Test func aCardShowsAtMostThreeConcerns() {
    let store = makeSetup()
    var reply = Fixtures.projectDraft
    reply.concerns += reply.concerns
    store.take(reply)
    #expect(store.concerns.count == 3)
}

@MainActor
@Test func usingAConcernWritesItsSuggestionAndPutsItAway() async {
    let store = makeSetup()
    store.paragraph = paragraph
    await store.draft()
    let concern = store.concerns(for: "done_when")[0]
    store.use(concern)
    #expect(store.drafts["done_when"]?.text == concern.suggestion)
    #expect(store.concerns(for: "done_when").isEmpty)
    #expect(store.concerns(for: "name").count == 1)
}

@MainActor
@Test func editingAFieldOrKeepingPutsItsConcernAway() async {
    let store = makeSetup()
    store.paragraph = paragraph
    await store.draft()
    store.drafts["done_when", default: WidgetDraft()].text = "The issue reached 400 people."
    #expect(store.concerns(for: "done_when").isEmpty)
    #expect(store.concerns(for: "name").count == 1)
    store.keep(store.concerns(for: "name")[0])
    #expect(store.concerns.isEmpty)
    #expect(store.drafts["name"]?.text == "Launch the newsletter")
}

@MainActor
@Test func anUnavailableDraftSeedsTheGoalAndAsksTheRest() async {
    let store = makeSetup(client: PreviewClient(modelUnavailable: true))
    #expect(store.primaryLabel == "Draft")
    store.paragraph = paragraph
    await store.primary()
    #expect(store.isFallback)
    #expect(
        store.fallbackLine
            == "I couldn\u{2019}t draft this (no model is set up; sign in to one in Settings), "
            + "so I\u{2019}ll ask you a few things one at a time.")
    #expect(store.drafts["goal"]?.text == paragraph)
    #expect(store.step == .asking("name"))
    #expect(store.progress == "A few more things")
    for (id, text) in [
        ("name", "Newsletter"), ("done_when", "The first issue is out."),
        ("context", "400 subscribers."), ("done_looks_like", "A first issue."),
    ] {
        #expect(store.step == .asking(id))
        store.drafts[id, default: WidgetDraft()].text = text
        if id == "done_looks_like" {
            store.drafts["target_date", default: WidgetDraft()].date =
                ProjectRequest.date(from: "2026-01-05")
        }
        await store.primary()
    }
    #expect(store.step == .card)
    #expect(store.concerns.map(\.field) == ["target_date"])
}

@MainActor
@Test func noModelStartsWithoutOne() async {
    let store = makeSetup(canDraft: false)
    #expect(store.isFallback)
    #expect(store.primaryLabel == "Next")
    #expect(
        store.fallbackLine
            == "No one is signed in to a model yet, so I\u{2019}ll ask you a few things one at a time."
    )
    store.paragraph = paragraph
    await store.primary()
    #expect(store.drafts["goal"]?.text == paragraph)
    #expect(store.drafts["name"]?.text ?? "" == "")
    #expect(store.step == .asking("name"))
}

@MainActor
@Test func theFormIsRememberedAndForgotten() async {
    let defaults = freshDefaults()
    let store = makeSetup(defaults: defaults)
    store.showForm()
    #expect(store.step == .form)
    #expect(defaults.bool(forKey: "setup.project.form"))
    #expect(makeSetup(defaults: defaults).step == .form)
    await store.showGuide()
    #expect(store.step == .opener)
    #expect(!defaults.bool(forKey: "setup.project.form"))
    #expect(makeSetup(defaults: defaults).step == .opener)
}

@MainActor
@Test func backKeepsATypedAnswer() async {
    let store = makeSetup(canDraft: false)
    store.paragraph = paragraph
    await store.draft()
    store.drafts["name", default: WidgetDraft()].text = "Newsletter"
    await store.next()
    #expect(store.step == .asking("done_when"))
    store.back()
    #expect(store.step == .asking("name"))
    #expect(store.drafts["name"]?.text == "Newsletter")
    store.back()
    #expect(store.step == .opener)
    #expect(store.paragraph == paragraph)
}

@MainActor
@Test func createSendsWhatTheCardShows() async throws {
    let store = makeSetup()
    store.paragraph = paragraph
    await store.draft()
    store.use(store.concerns(for: "done_when")[0])
    store.drafts["context", default: WidgetDraft()].text = "  Started in March.  "
    await store.primary()
    guard case .project(let project) = try #require(store.outcome) else { return }
    #expect(project.name == "Launch the newsletter")
    #expect(project.brief.doneWhen == "The first issue has reached every address on the list")
    #expect(project.brief.context == "Started in March.")
    #expect(project.brief.goal == Fixtures.projectDraft.brief.goal)
    #expect(project.targetDate == "2026-11-20")
    #expect(store.saveError == nil)
}

@MainActor
@Test func editingOpensTheFormAndSaves() async throws {
    let project = Fixtures.projects[0]
    let store = makeSetup(editing: project)
    #expect(store.step == .form)
    #expect(store.primaryLabel == "Save")
    #expect(!store.canGoBack)
    #expect(store.canContinue)
    #expect(store.drafts["target_date"]?.date.map(ProjectRequest.text(from:)) == "2026-11-20")
    store.drafts["name", default: WidgetDraft()].text = "Newsletter, first issue"
    await store.primary()
    guard case .project(let saved) = try #require(store.outcome) else { return }
    #expect(saved.id == project.id)
    #expect(saved.name == "Newsletter, first issue")
    #expect(saved.brief == project.brief)
    #expect(saved.targetDate == "2026-11-20")
}

@Test func theProjectScriptHoldsTogether() {
    let fields = SetupScript.project.fields
    #expect(Set(fields.map(\.id)).count == fields.count)
    let firstOptional = fields.firstIndex { !$0.required } ?? fields.count
    #expect(fields[firstOptional...].allSatisfy { !$0.required })
    #expect(fields.contains { $0.id == SetupScript.project.opener.seeds })
    for field in fields {
        var draft = WidgetDraft()
        if case .date = field.widget.kind {
            draft.date = ProjectRequest.date(from: field.example)
        } else {
            draft.text = field.example
        }
        #expect(draft.isValid(for: field.widget), "\(field.id)")
        #expect(!draft.isEmpty(for: field.widget), "\(field.id)")
    }
}
