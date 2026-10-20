// Store tests for a deliverable's window over real files in a temp folder: a
// file that decodes is shown with its title and the page and PDF beside it
// noted, a file that does not decode, is gone, or is too large is unreadable,
// and a reload shows what the bot rewrote.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

/// A folder with `files` in it, and the spec's path.
private func folder(_ files: [String: String]) throws -> (spec: String, root: URL) {
    let root = FileManager.default.temporaryDirectory
        .appending(path: "deliverable-store-\(UUID().uuidString)")
    try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    for (name, text) in files {
        try Data(text.utf8).write(to: root.appending(path: name))
    }
    return (root.appending(path: "Launch.deliverable.json").path, root)
}

@MainActor
@Test func aDeliverableIsShownWithItsTitleAndSiblings() async throws {
    let made = try folder([
        "Launch.deliverable.json": Fixtures.kitchenSinkJSON, "Launch.html": "<html>",
    ])
    defer { try? FileManager.default.removeItem(at: made.root) }
    let store = DeliverableStore(path: made.spec)
    #expect(store.state == .loading)
    await store.load()
    #expect(store.state == .shown(Fixtures.kitchenSink))
    #expect(store.title == Fixtures.kitchenSink.title)
    #expect(store.hasPage && !store.hasPDF)
    #expect(store.pagePath == made.root.appending(path: "Launch.html").path)
    #expect(store.pdfPath == made.root.appending(path: "Launch.pdf").path)
}

@MainActor
@Test func aFileThatDoesNotDecodeIsUnreadable() async throws {
    let made = try folder([
        "Launch.deliverable.json": #"{"kind": "dashboard", "version": 1, "x": 1}"#
    ])
    defer { try? FileManager.default.removeItem(at: made.root) }
    let store = DeliverableStore(path: made.spec)
    await store.load()
    #expect(store.state == .unreadable)
    #expect(store.title == "Launch")
}

@MainActor
@Test func aMissingOrOversizedFileIsUnreadable() async throws {
    let gone = DeliverableStore(path: "/nonexistent/Gone.deliverable.json")
    await gone.load()
    #expect(gone.state == .unreadable)
    let big = String(repeating: " ", count: DeliverableStore.sizeCap + 1)
    let made = try folder(["Launch.deliverable.json": big])
    defer { try? FileManager.default.removeItem(at: made.root) }
    let store = DeliverableStore(path: made.spec)
    await store.load()
    #expect(store.state == .unreadable)
}

@MainActor
@Test func aReloadShowsWhatWasRewritten() async throws {
    let made = try folder(["Launch.deliverable.json": Fixtures.kitchenSinkJSON])
    defer { try? FileManager.default.removeItem(at: made.root) }
    let store = DeliverableStore(path: made.spec)
    await store.load()
    try Data(Fixtures.newerDeliverableJSON.utf8).write(to: URL(filePath: made.spec))
    await store.load()
    #expect(store.state == .shown(Fixtures.newerDeliverable))
    try Data("not json".utf8).write(to: URL(filePath: made.spec))
    await store.load()
    #expect(store.state == .unreadable)
}

@MainActor
@Test func aStoreGivenItsStateKeepsIt() async {
    let store = DeliverableStore(
        path: "/nowhere/A.deliverable.json", previewing: Fixtures.kitchenSink, hasPage: true,
        hasPDF: true)
    await store.load()
    #expect(store.state == .shown(Fixtures.kitchenSink))
}
