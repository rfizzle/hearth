// Store tests for the introduction card's two seams in a conversation: a chip
// filling the composer's draft, and the "About" header's open state.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func store() -> ConversationStore {
    ConversationStore(
        client: PreviewClient(), relay: EventRelay(client: PreviewClient()),
        conversationID: Fixtures.dmConversation.id)
}

@MainActor
@Test func aChipFillsTheComposerReplacingTheDraftAndAsksForFocus() {
    let store = store()
    let chip = Fixtures.chiefOfStaffIntroduction.tryAsking
    #expect(store.draft.isEmpty)
    let before = store.focusComposer

    store.fill(chip[0])
    #expect(store.draft == chip[0])
    #expect(store.focusComposer == before + 1)

    store.fill(chip[2])
    #expect(store.draft == chip[2])
    #expect(store.focusComposer == before + 2)
}

@MainActor
@Test func fillingNeverSendsAnything() async {
    let store = store()
    await store.load()
    let count = store.messages.count
    store.fill("Summarise this week's customer calls")
    #expect(store.messages.count == count)
    #expect(store.run == nil)
}

@MainActor
@Test func theAboutHeaderStartsClosedAndKeepsItsOpenState() {
    let store = store()
    #expect(!store.aboutOpen)
    store.aboutOpen = true
    #expect(store.aboutOpen)
    store.aboutOpen = false
    #expect(!store.aboutOpen)
}

@Test func everyFixtureBotCarriesThreeLinesAndThreePrompts() {
    for bot in [Fixtures.chiefOfStaff, Fixtures.bot] {
        #expect(!bot.introduction.purpose.isEmpty, "\(bot.name)")
        #expect(bot.introduction.how.count == 3, "\(bot.name)")
        #expect(bot.introduction.tryAsking.count == 3, "\(bot.name)")
        #expect(bot.introduction.tryAsking.allSatisfy { $0.count <= 48 }, "\(bot.name)")
    }
}
