// Store tests for the inspector's notes: PreviewClient's five notes with the
// pinned one first, the three calls, the scripted note changes and the count.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func loadedStore() async -> NotesStore {
    let client = PreviewClient()
    let store = NotesStore(
        client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id)
    await store.load()
    return store
}

/// The scripted change events that carry a note, in order.
private func noteEvents() -> [Hearth_V1_SubscribeResponse] {
    Fixtures.memoryEvents.filter {
        if case .noteChanged = $0.event { true } else { false }
    }
}

@MainActor
@Test func theFixtureNotesListWithThePinnedOneFirstAndTheCountFollows() async {
    let store = await loadedStore()
    #expect(store.loaded)
    #expect(store.notes.map(\.id) == Fixtures.notes.map(\.id))
    #expect(store.notes.first?.pinned == true)
    #expect(store.cap == Int(Fixtures.noteCap))
    #expect(store.countText == "5 of 40")
}

@MainActor
@Test func aNoteForAnotherBotListsNothing() async {
    let client = PreviewClient()
    let store = NotesStore(client: client, relay: EventRelay(client: client), botID: "nobody")
    await store.load()
    #expect(store.loaded)
    #expect(store.notes.isEmpty)
}

@MainActor
@Test func pinningMovesANoteToTheTopAndUnpinningLetsItGo() async {
    let store = await loadedStore()
    await store.pin(Fixtures.notes[2], true)
    #expect(store.notes.map(\.id) == ["note-1", "note-3", "note-2", "note-4", "note-5"])
    #expect(store.notes[1].pinned)
    await store.pin(Fixtures.notes[0], false)
    #expect(store.notes.map(\.id) == ["note-3", "note-1", "note-2", "note-4", "note-5"])
}

@MainActor
@Test func editingReplacesTheWordsInPlace() async {
    let store = await loadedStore()
    await store.edit(Fixtures.notes[3], text: "Send briefs as plain text.")
    #expect(store.notes[3].id == "note-4")
    #expect(store.notes[3].text == "Send briefs as plain text.")
    #expect(store.error == nil)
}

@MainActor
@Test func deletingDropsTheNoteAndTheCountFollows() async {
    let store = await loadedStore()
    await store.delete(Fixtures.notes[1])
    #expect(store.notes.map(\.id) == ["note-1", "note-3", "note-4", "note-5"])
    #expect(store.countText == "4 of 40")
}

@MainActor
@Test func theScriptedChangesAddEditAndDeleteNotes() async {
    let store = await loadedStore()
    let events = noteEvents()
    #expect(events.count == 3)

    await store.handle(events[0])
    #expect(
        store.notes.map(\.id) == ["note-1", "note-new", "note-2", "note-3", "note-4", "note-5"])
    #expect(store.countText == "6 of 40")

    await store.handle(events[1])
    #expect(
        store.notes.first { $0.id == "note-2" }?.text
            == "Our product is an AI writing tool for teams of five to fifty.")
    #expect(store.notes.count == 6)

    await store.handle(events[2])
    #expect(!store.notes.contains { $0.id == "note-5" })
    #expect(store.countText == "5 of 40")
}

@MainActor
@Test func anotherBotsChangeIsIgnored() async {
    let store = await loadedStore()
    var event = noteEvents()[0]
    guard case .noteChanged(var change)? = event.event else { return }
    change.botID = "someone-else"
    event.event = .noteChanged(change)
    await store.handle(event)
    #expect(store.notes.count == 5)
}

@MainActor
@Test func aResyncRefetchesTheNotes() async {
    let store = await loadedStore()
    await store.delete(Fixtures.notes[0])
    #expect(store.notes.count == 4)
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.notes.count == 5)
}
