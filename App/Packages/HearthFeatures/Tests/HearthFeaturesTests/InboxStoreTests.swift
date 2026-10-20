// Store tests for the Inbox: PreviewClient's four items in two sections, the
// four calls and what each card shows until the core's change removes it,
// Always Allow offered from the grants, and a resync refetching.

import Foundation
import HearthDesign
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

@MainActor
private func loadedStore(relay: EventRelay? = nil) async -> InboxStore {
    let store = InboxStore(
        client: PreviewClient(), relay: relay ?? EventRelay(client: PreviewClient()))
    await store.load()
    return store
}

/// The relay's change for `item`.
private func changed(_ item: Hearth_V1_InboxItem) -> Hearth_V1_SubscribeResponse {
    .with { $0.event = .inboxChanged(.with { $0.item = item }) }
}

private func closed(_ item: Hearth_V1_InboxItem) -> Hearth_V1_InboxItem {
    var item = item
    item.state = .closed
    return item
}

@MainActor
@Test func theFixtureInboxListsFourItemsInTwoSectionsNewestFirst() async {
    let store = await loadedStore()
    #expect(store.loaded)
    #expect(store.openCount == 4)
    #expect(
        store.items.map(\.id) == [
            "inbox-approval", "inbox-widget-question", "inbox-question", "inbox-failure",
        ])
    #expect(store.sections == [.needsDecision, .toNote])
    #expect(store.items(in: .needsDecision).map(\.kind) == [.approval, .question, .question])
    #expect(store.items(in: .toNote).map(\.kind) == [.failure])
    #expect(InboxStore.Section.needsDecision.rawValue == "Needs a decision")
    #expect(InboxStore.Section.toNote.rawValue == "To note")
}

@MainActor
@Test func allowOnceAsksTheCoreAndTheClosingChangeRemovesTheCard() async {
    let store = await loadedStore()
    let item = Fixtures.approval
    #expect(store.state(of: item) == .open)
    await store.decide(item, .allowOnce)
    // PreviewClient answers as the core would: closed, with its outcome.
    #expect(store.answered[item.id]?.decision == .allowOnce)
    #expect(store.state(of: item) == .closed("Allowed", .positive))
    #expect(store.openCount == 4)

    await store.handle(changed(closed(item)))
    #expect(store.openCount == 3)
    #expect(store.openItem(forMessage: item.messageID) == nil)
    #expect(store.answered.isEmpty)
    #expect(store.sections == [.needsDecision, .toNote])
}

@MainActor
@Test func aDenyCarriesItsReasonAndShowsQuietly() async {
    let store = await loadedStore()
    await store.decide(Fixtures.approval, .deny, reason: "Keep it")
    #expect(store.state(of: Fixtures.approval) == .closed("Denied", .neutral))
    // A second answer to an answered card is not sent.
    await store.decide(Fixtures.approval, .allowOnce)
    #expect(store.answered[Fixtures.approval.id]?.decision == .deny)
}

@MainActor
@Test func aQuestionTakesASuggestionOrTypedWords() async {
    let suggested = Fixtures.question.suggestedAnswers[1]
    let store = await loadedStore()
    await store.answer(itemID: Fixtures.question.id, text: suggested)
    #expect(store.answered[Fixtures.question.id]?.answer == suggested)
    #expect(store.state(of: Fixtures.question) == .closed("Answered", .positive))

    let typed = await loadedStore()
    await typed.answer(
        itemID: Fixtures.question.id, text: "Air-source, and a line on ground-source")
    #expect(
        typed.answered[Fixtures.question.id]?.answer == "Air-source, and a line on ground-source")
    await typed.handle(changed(closed(Fixtures.question)))
    #expect(typed.items(in: .needsDecision).map(\.kind) == [.approval, .question])
}

@MainActor
@Test func aFailureCanBeRetriedOrDismissed() async {
    let retried = await loadedStore()
    await retried.retry(Fixtures.failure)
    #expect(retried.state(of: Fixtures.failure) == .closed("Trying again", .neutral))
    await retried.handle(changed(closed(Fixtures.failure)))
    #expect(retried.sections == [.needsDecision])

    let dismissed = await loadedStore()
    await dismissed.acknowledge(Fixtures.failure)
    #expect(dismissed.state(of: Fixtures.failure) == .closed("Acknowledged", .neutral))
    await dismissed.handle(changed(closed(Fixtures.failure)))
    #expect(dismissed.openCount == 3)
}

@MainActor
@Test func alwaysAllowIsOfferedOnlyWhereTheGrantCanStand() async {
    let store = await loadedStore()
    #expect(!store.canAlwaysAllow(Fixtures.approval))
    var write = Fixtures.approval
    write.id = "inbox-write"
    write.call.name = "files.write"
    #expect(store.canAlwaysAllow(write))
    write.call.name = "calendar.add"
    #expect(!store.canAlwaysAllow(write))
    #expect(!store.canAlwaysAllow(Fixtures.question))
}

@MainActor
@Test func grantsFollowTheirChangesAndAResyncRefetchesThem() async {
    let store = await loadedStore()
    var write = Fixtures.approval
    write.call.name = "files.write"
    #expect(store.canAlwaysAllow(write))
    let locked = Fixtures.grants.map { grant -> Hearth_V1_Grant in
        var grant = grant
        grant.canAllow = false
        return grant
    }
    await store.handle(
        .with {
            $0.event = .grantsChanged(
                .with {
                    $0.botID = Fixtures.bot.id
                    $0.grants = locked
                })
        })
    #expect(!store.canAlwaysAllow(write))
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.canAlwaysAllow(write))
}

@MainActor
@Test func anOpenChangeJoinsNewestFirstAndTheCountFollows() async {
    let store = await loadedStore()
    var newer = Fixtures.approval
    newer.id = "inbox-newer"
    newer.messageID = "message-30"
    newer.createdAt = .init(date: Fixtures.approval.createdAt.date.addingTimeInterval(60))
    await store.handle(changed(newer))
    #expect(store.openCount == 5)
    #expect(store.items.first?.id == "inbox-newer")
    // The same change again replaces it rather than adding a second.
    await store.handle(changed(newer))
    #expect(store.openCount == 5)
}

@MainActor
@Test func aResyncFromTheRelayRefetchesTheList() async throws {
    let relay = EventRelay(subscribe: { _ in
        AsyncThrowingStream { continuation in
            continuation.yield(
                .with {
                    $0.sequence = 4
                    $0.event = .resync(Hearth_V1_Resync())
                })
        }
    })
    let store = await loadedStore(relay: relay)
    await store.handle(changed(closed(Fixtures.approval)))
    await store.handle(changed(closed(Fixtures.question)))
    #expect(store.openCount == 2)

    relay.start()
    for _ in 0..<400 where store.openCount != 4 {
        try await Task.sleep(for: .milliseconds(5))
    }
    relay.stop()
    #expect(store.openCount == 4)
    #expect(relay.lastSequence == 4)
}
