// Store tests for the Access sheet, against PreviewClient's five grants and a
// scripted change event.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func store() -> GrantsStore {
    let client = PreviewClient()
    return GrantsStore(
        client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id)
}

private let extraFolder = "/Users/preview/Documents/Pricing"

@MainActor
@Test func theRowsComeFromTheFixtureGrantsInTheCoresOrder() async {
    let store = store()
    await store.load()
    #expect(
        store.rows.map(\.capability) == [
            .filesRead, .filesWrite, .filesDelete, .webFetch, .webSearch,
        ])
    #expect(store.rows.map(\.mode) == [.allowed, .ask, .ask, .ask, .ask])
    #expect(store.rows.map(\.title) == Fixtures.grants.map(\.title))
    #expect(store.folders == [.init(path: Fixtures.bot.folderPath, isOwn: true)])
    #expect(store.grants == Fixtures.grants)
}

@MainActor
@Test func changingAModeEditsOneRow() async {
    let store = store()
    await store.load()
    store.set(mode: .notGranted, for: .webSearch)
    store.set(mode: .allowed, for: .filesWrite)
    #expect(store.mode(for: .webSearch) == .notGranted)
    #expect(store.mode(for: .filesWrite) == .allowed)
    #expect(store.mode(for: .filesRead) == .allowed)
}

@MainActor
@Test func allowedIsRefusedWhereTheOwnerMustBeAsked() async {
    let store = store()
    await store.load()
    #expect(store.rows.first { $0.capability == .filesDelete }?.canAllow == false)
    store.set(mode: .allowed, for: .filesDelete)
    #expect(store.mode(for: .filesDelete) == .ask)
    store.set(mode: .notGranted, for: .filesDelete)
    #expect(store.mode(for: .filesDelete) == .notGranted)
}

@MainActor
@Test func anAddedFolderGetsReadWriteAndDeleteAtTheCurrentModes() async {
    let store = store()
    await store.load()
    store.set(mode: .notGranted, for: .filesWrite)
    store.addFolder(extraFolder)
    store.addFolder(extraFolder)
    store.addFolder(Fixtures.bot.folderPath)
    #expect(store.folders.map(\.path) == [Fixtures.bot.folderPath, extraFolder])
    let added = store.grants.filter { $0.folder == extraFolder }
    #expect(added.map(\.capability) == [.filesRead, .filesWrite, .filesDelete])
    #expect(added.map(\.mode) == [.allowed, .notGranted, .ask])
    #expect(added.allSatisfy { !$0.ownFolder })
    #expect(store.grants.filter { $0.folder.isEmpty }.count == 2)
}

@MainActor
@Test func removingAFolderDropsItsRowsAndTheOwnFolderStays() async {
    let store = store()
    await store.load()
    store.addFolder(extraFolder)
    store.removeFolder(extraFolder)
    store.removeFolder(Fixtures.bot.folderPath)
    #expect(store.folders == [.init(path: Fixtures.bot.folderPath, isOwn: true)])
    #expect(store.grants == Fixtures.grants)
}

@MainActor
@Test func saveSendsEveryRowAndTheOwnFolder() async {
    let store = store()
    await store.load()
    store.set(mode: .notGranted, for: .webFetch)
    store.addFolder(extraFolder)
    let shown = store.grants
    #expect(shown.count == 3 * 2 + 2)
    #expect(shown.filter(\.ownFolder).count == 3)
    await store.save()
    // PreviewClient answers with what it was sent, so the store holds exactly it.
    #expect(store.grants == shown)
    #expect(store.didSave)
    #expect(store.error == nil)
}

@MainActor
@Test func cancelDiscardsTheEdits() async {
    let store = store()
    await store.load()
    store.set(mode: .notGranted, for: .filesRead)
    store.addFolder(extraFolder)
    store.discard()
    #expect(store.grants == Fixtures.grants)
    #expect(!store.didSave)
}

@MainActor
@Test func aGrantsChangedEventReplacesTheRows() async {
    let store = store()
    await store.load()
    store.set(mode: .notGranted, for: .webSearch)
    var changed = Fixtures.grants
    changed[3].mode = .allowed
    let event = Hearth_V1_SubscribeResponse.with {
        $0.event = .grantsChanged(
            .with {
                $0.botID = Fixtures.bot.id
                $0.grants = changed
            })
    }
    await store.handle(event)
    #expect(store.grants == changed)
    #expect(store.mode(for: .webSearch) == .ask)

    // Another bot's change is not this sheet's.
    let other = Hearth_V1_SubscribeResponse.with {
        $0.event = .grantsChanged(.with { $0.botID = "bot-other" })
    }
    await store.handle(other)
    #expect(store.grants == changed)
}
