// Store tests for the inspector's handbook: PreviewClient's two pages and the
// meter, a save sending the title, Markdown and scope, the core's refusal, the
// delete, and the scripted page changes.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func loadedHandbook() async -> HandbookStore {
    let client = PreviewClient()
    let store = HandbookStore(
        client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id,
        botName: Fixtures.bot.name)
    await store.load()
    return store
}

/// The scripted change events that carry a page, in order.
private func pageEvents() -> [Hearth_V1_SubscribeResponse] {
    Fixtures.memoryEvents.filter {
        if case .handbookChanged = $0.event { true } else { false }
    }
}

@MainActor
@Test func theFixturePagesAndMeterLoadWithTheirScopeWords() async {
    let store = await loadedHandbook()
    #expect(store.loaded)
    #expect(store.pages.map(\.id) == ["page-house-style", "page-sources"])
    #expect(store.meter.usedBytes == Fixtures.handbookMeter.usedBytes)
    #expect(store.meter.capBytes == 24 * 1024)
    #expect(store.scopeText(store.pages[0]) == "Everyone")
    #expect(store.scopeText(store.pages[1]) == Fixtures.bot.name)
}

@MainActor
@Test func aBotWithNoPagesOfItsOwnReadsOnlyThePagesForEveryone() async {
    let client = PreviewClient()
    let store = HandbookStore(
        client: client, relay: EventRelay(client: client), botID: "nobody", botName: "Nobody")
    await store.load()
    #expect(store.pages.map(\.id) == ["page-house-style"])
}

@MainActor
@Test func savingSendsTheTitleMarkdownAndScopeAndAddsThePage() async {
    let store = await loadedHandbook()
    let scope = Hearth_V1_Scope.with { $0.botID = Fixtures.bot.id }
    let saved = await store.put(title: "Tone", markdown: "Be brief.", scope: scope)
    #expect(saved)
    #expect(store.saveError == nil)
    let page = store.pages.last
    #expect(page?.id == "page-new")
    #expect(page?.title == "Tone")
    #expect(page?.markdown == "Be brief.")
    #expect(page?.scope.botID == Fixtures.bot.id)
    #expect(page?.bytes == 9)
}

@MainActor
@Test func savingAnExistingPageReplacesItInPlace() async {
    let store = await loadedHandbook()
    let first = store.pages[0]
    await store.put(
        id: first.id, title: "House rules", markdown: first.markdown, scope: first.scope)
    #expect(store.pages.map(\.id) == ["page-house-style", "page-sources"])
    #expect(store.pages[0].title == "House rules")
}

@MainActor
@Test func theCoresRefusalShowsInItsWordsAndNothingIsAdded() async {
    let store = await loadedHandbook()
    let saved = await store.put(
        title: "  ", markdown: "text", scope: .with { $0.everyone = true })
    #expect(!saved)
    #expect(store.saveError == "a page needs a title")
    #expect(store.pages.count == 2)
    store.clearSaveError()
    #expect(store.saveError == nil)
}

@MainActor
@Test func deletingDropsThePage() async {
    let store = await loadedHandbook()
    await store.delete(store.pages[0])
    #expect(store.pages.map(\.id) == ["page-sources"])
    #expect(store.error == nil)
}

@MainActor
@Test func theScriptedChangesReplaceThenDeleteAPageAndTheMeterFollows() async {
    let store = await loadedHandbook()
    let events = pageEvents()
    #expect(events.count == 2)
    let used = store.meter.usedBytes
    let sources = store.pages[1].bytes

    await store.handle(events[0])
    #expect(store.pages.count == 2)
    #expect(store.meter.usedBytes == used)

    await store.handle(events[1])
    #expect(store.pages.map(\.id) == ["page-house-style"])
    #expect(store.meter.usedBytes == used - sources)
}

@MainActor
@Test func aPageChangedToAnotherBotsLeavesAndAnotherBotsNewPageIsNotAdded() async {
    let store = await loadedHandbook()
    var moved = Fixtures.handbookPages[1]
    moved.scope = .with { $0.botID = "someone-else" }
    await store.handle(
        .with { $0.event = .handbookChanged(.with { $0.page = moved }) })
    #expect(store.pages.map(\.id) == ["page-house-style"])

    var fresh = moved
    fresh.id = "page-elsewhere"
    await store.handle(
        .with { $0.event = .handbookChanged(.with { $0.page = fresh }) })
    #expect(store.pages.count == 1)
}

@MainActor
@Test func aResyncRefetchesThePages() async {
    let store = await loadedHandbook()
    await store.delete(store.pages[0])
    #expect(store.pages.count == 1)
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.pages.count == 2)
}
