// Store tests for a dashboard that stays live over PreviewClient: bound values
// load, an event the core would send makes the next read differ, a measure the
// core cannot read shows its reason, a resync reads again at once, and a spec
// with nothing bound never asks.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

private let projectChanged = Fixtures.measureEvents[0]

/// A store over a temp file holding `json`, over `client`.
@MainActor
private func liveStore(
    _ json: String, client: PreviewClient, relay: EventRelay? = nil
) throws -> (store: DeliverableStore, root: URL) {
    let root = FileManager.default.temporaryDirectory
        .appending(path: "deliverable-live-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    let spec = root.appending(path: "Launch.deliverable.json")
    try Data(json.utf8).write(to: spec)
    let store = DeliverableStore(path: spec.path, client: client, relay: relay)
    store.debounce = .milliseconds(10)
    return (store, root)
}

@MainActor
@Test func boundValuesLoadWithTheFile() async throws {
    let client = PreviewClient()
    let made = try liveStore(Fixtures.kitchenSinkJSON, client: client)
    defer { try? FileManager.default.removeItem(at: made.root) }
    #expect(!made.store.isLive)
    await made.store.load()
    let tasks = made.store.values[Fixtures.tasksMeasure]
    #expect(tasks?.done == 3 && tasks?.total == 7)
    #expect(made.store.values.count == 3)
    #expect(made.store.isLive)
    #expect(client.measureReads.all.count == 1)
}

@MainActor
@Test func aProjectEventMakesTheNextReadFollow() async throws {
    let client = PreviewClient()
    let made = try liveStore(Fixtures.kitchenSinkJSON, client: client)
    defer { try? FileManager.default.removeItem(at: made.root) }
    await made.store.load()
    client.deliver(projectChanged)
    #expect(made.store.values[Fixtures.tasksMeasure]?.done == 3)
    await made.store.handle(projectChanged)
    await made.store.handle(projectChanged)
    await made.store.settle()
    #expect(made.store.values[Fixtures.tasksMeasure]?.done == 4)
    // Two events inside the wait are one read.
    #expect(client.measureReads.all.count == 2)
}

@MainActor
@Test func onlyTheEventsThatMoveAFigureReadAgain() async throws {
    let client = PreviewClient()
    let made = try liveStore(Fixtures.kitchenSinkJSON, client: client)
    defer { try? FileManager.default.removeItem(at: made.root) }
    await made.store.load()
    await made.store.handle(.with { $0.event = .heartbeat(.init()) })
    await made.store.settle()
    #expect(client.measureReads.all.count == 1)
    await made.store.handle(.with { $0.event = .usageTick(.init()) })
    await made.store.settle()
    #expect(client.measureReads.all.count == 2)
}

@MainActor
@Test func aMissingMeasureShowsItsReason() async throws {
    let json = Fixtures.kitchenSinkJSON.replacingOccurrences(
        of: Fixtures.tasksMeasure, with: Fixtures.goneProjectMeasure)
    let made = try liveStore(json, client: PreviewClient())
    defer { try? FileManager.default.removeItem(at: made.root) }
    await made.store.load()
    #expect(made.store.values[Fixtures.goneProjectMeasure]?.missing == "No such project")
    #expect(made.store.isLive)
}

@MainActor
@Test func aResyncReadsAtOnce() async throws {
    let client = PreviewClient()
    let made = try liveStore(Fixtures.kitchenSinkJSON, client: client)
    defer { try? FileManager.default.removeItem(at: made.root) }
    await made.store.load()
    client.deliver(projectChanged)
    await made.store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(made.store.values[Fixtures.tasksMeasure]?.done == 4)
}

@MainActor
@Test func aSpecWithNothingBoundNeverAsks() async throws {
    let client = PreviewClient()
    let made = try liveStore(Fixtures.newerDeliverableJSON, client: client)
    defer { try? FileManager.default.removeItem(at: made.root) }
    await made.store.load()
    await made.store.handle(projectChanged)
    await made.store.settle()
    #expect(client.measureReads.all.isEmpty)
    #expect(!made.store.isLive)
}

@Test func readsCarryAtMostAHundredIds() {
    #expect(DeliverableStore.readCap == 100)
}
