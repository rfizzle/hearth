// Store tests for the deliverable templates against PreviewClient: the list
// splits shipped from the owner's, a save appends, a refusal shows in words, a
// delete removes, a template that fails to load keeps its problem; and the
// document a deliverable becomes when it is saved as a template.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
@Test func theListSplitsShippedFromTheOwners() async {
    let client = PreviewClient()
    client.keepTemplate(
        .with {
            $0.id = "mine"; $0.name = "Mine"
        })
    let store = TemplatesStore(client: client)
    await store.load()
    #expect(store.shipped.map(\.id) == Fixtures.shippedDeliverableIDs)
    #expect(store.owned.map(\.id) == ["mine"])
}

@MainActor
@Test func savingATemplateAppendsItAndRefusalShowsInWords() async throws {
    let store = TemplatesStore(client: PreviewClient())
    let data = Data(Fixtures.kitchenSinkJSON.utf8)
    let json = try #require(
        TemplateDocument.json(from: data, id: "office-week", name: "Office week", when: "Weekly."))
    #expect(await store.save(json: json))
    #expect(store.owned.map(\.id) == ["office-week"])
    #expect(store.error == nil)

    let shipped = try #require(
        TemplateDocument.json(from: data, id: "report", name: "Report", when: "x"))
    #expect(await !store.save(json: shipped))
    #expect(store.error == "report is a shipped template; pick another id")
    #expect(store.owned.map(\.id) == ["office-week"])
}

@MainActor
@Test func deletingATemplateRemovesIt() async {
    let client = PreviewClient()
    client.keepTemplate(.with { $0.id = "mine" })
    let store = TemplatesStore(client: client)
    await store.load()
    await store.delete(id: "mine")
    #expect(store.owned.isEmpty)
    await store.delete(id: "mine")
    #expect(store.error == "there is no template mine")
}

@MainActor
@Test func aTemplateThatFailsToLoadKeepsItsProblem() async {
    let client = PreviewClient()
    client.keepTemplate(
        .with {
            $0.id = "bad"; $0.problem = "section 1: nothing allowed"
        })
    let store = TemplatesStore(client: client)
    await store.load()
    #expect(store.owned.first?.problem == "section 1: nothing allowed")
}

@MainActor
@Test func theFolderComesFromTheFirstOwnedPath() async {
    let client = PreviewClient()
    client.keepTemplate(
        .with {
            $0.id = "mine"; $0.path = "/Hearth/Templates/mine.deliverable.json"
        })
    let store = TemplatesStore(client: client)
    await store.load()
    #expect(store.folderPath == "/Hearth/Templates")
}

@Test func aNameMakesAnId() {
    #expect(TemplateDocument.slug("Office week!") == "office-week")
    #expect(TemplateDocument.slug("  Café  --  Menu 2 ") == "cafe-menu-2")
    #expect(TemplateDocument.slug("???") == "")
    #expect(TemplateDocument.slug(String(repeating: "ab ", count: 30)).count <= 40)
}

@Test func aDeliverableBecomesATemplate() throws {
    let data = Data(Fixtures.kitchenSinkJSON.utf8)
    let text = try #require(
        TemplateDocument.json(from: data, id: "office-week", name: "Office week", when: "Weekly."))
    let top = try #require(
        try JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any])
    let head = try #require(top["template"] as? [String: String])
    #expect(head == ["id": "office-week", "name": "Office week", "when": "Weekly."])
    let sections = try #require(top["sections"] as? [[String: Any]])
    #expect(sections[0]["allow"] as? [String] == ["metric", "progress", "light"])
    #expect(sections[1]["allow"] as? [String] == ["line", "bar", "pie"])
    #expect(sections.allSatisfy { ($0["hint"] as? String) == "" })
    let template = try Deliverable(json: Data(text.utf8))
    #expect(template.sections.map(\.widgets.count) == [3, 3, 2, 2])
    #expect(template.template?.id == "office-week")
}

@Test func somethingThatIsNotADeliverableIsNotATemplate() {
    #expect(TemplateDocument.json(from: Data("[1]".utf8), id: "a", name: "A", when: "") == nil)
}
