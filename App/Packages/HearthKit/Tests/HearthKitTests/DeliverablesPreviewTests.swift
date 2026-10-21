// PreviewClient serves the office's figures and the owner's templates the way
// the core does: fixture values, a missing one, and a project change that
// moves the next read.

import Testing

@testable import HearthKit

private let tasks = "project.tasks/project-launch"

@Test func measuresReadTheLaunchAtThreeOfSevenACostAnInboxCountAndAMissingOne() async throws {
    let values = try await PreviewClient().readMeasures(ids: [
        tasks, "project.light/project-launch", "usage.cost/org/2026-10", "inbox.open",
        Fixtures.goneProjectMeasure,
    ])
    #expect(values.map(\.id).first == tasks)
    #expect(values[0].kind == .progress && values[0].done == 3 && values[0].total == 7)
    #expect(values[1].kind == .light && values[1].tone == .attention)
    #expect(values[2].number == Fixtures.orgCostMicros && values[2].unit == "usd_micros")
    #expect(values[3].number == Fixtures.inboxOpen)
    #expect(values[4].missing == "No such project")
    #expect(values.allSatisfy { $0.hasAsOf })
}

@Test func aProjectChangedEventMovesTheNextRead() async throws {
    let client = PreviewClient()
    for event in Fixtures.measureEvents { client.deliver(event) }
    let value = try await client.readMeasures(ids: [tasks])[0]
    #expect(value.done == 4 && value.total == 7)
}

@Test func theMeasureListNamesEveryIdWithNoValue() async throws {
    let list = try await PreviewClient().listMeasures()
    #expect(list.contains { $0.id == tasks && $0.label.hasPrefix("Tasks done") })
    #expect(list.allSatisfy { !$0.hasAsOf && $0.missing.isEmpty && $0.done == 0 })
}

@Test func moreThanAHundredMeasureIdsAreRefused() async {
    await #expect(throws: (any Error).self) {
        _ = try await PreviewClient().readMeasures(ids: Array(repeating: "inbox.open", count: 101))
    }
}

@Test func templatesListTheShippedFiveAndKeepWhatIsSavedAndDeleted() async throws {
    let client = PreviewClient()
    #expect(try await client.listDeliverableTemplates().count == 5)
    let json = #"{"template":{"id":"mine","name":"Mine","when":"When I say so."}}"#
    let saved = try await client.saveDeliverableTemplate(json: json)
    #expect(saved.id == "mine" && !saved.shipped && saved.json == json)
    #expect(try await client.listDeliverableTemplates().map(\.id).last == "mine")
    await #expect(throws: (any Error).self) {
        _ = try await client.saveDeliverableTemplate(
            json: #"{"template":{"id":"report","name":"R","when":"W"}}"#)
    }
    try await client.deleteDeliverableTemplate(id: "mine")
    #expect(try await client.listDeliverableTemplates().count == 5)
    await #expect(throws: (any Error).self) {
        try await client.deleteDeliverableTemplate(id: "mine")
    }
}
