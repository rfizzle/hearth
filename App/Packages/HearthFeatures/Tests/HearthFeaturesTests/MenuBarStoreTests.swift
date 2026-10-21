// Store tests for the menu bar extra over PreviewClient and the scripted
// events: a working bot appears and leaves, a failed run lands in "Needs you"
// as its failure item, a resync rebuilds both, and the quick ask sends into
// the chosen bot's DM and shows the newest two messages as the reply streams.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

/// A menu bar store and the stores under it, over one relay.
@MainActor
private struct Rig {
    let store: MenuBarStore
    let conversations: Conversations

    /// Hands each event to the stores the way the relay does, in order.
    func deliver(_ events: [Hearth_V1_SubscribeResponse]) async {
        for event in events {
            await store.bots.handle(event)
            await store.inbox.handle(event)
            for conversation in conversations.made.values { await conversation.handle(event) }
        }
    }
}

/// The DM stores the quick ask asked for, one per conversation.
@MainActor
private final class Conversations {
    var made: [String: ConversationStore] = [:]
}

@MainActor
private func rig(
    client: PreviewClient = PreviewClient(), relay: EventRelay? = nil
) async -> Rig {
    let relay = relay ?? EventRelay(client: client)
    let inbox = InboxStore(client: client, relay: relay)
    let conversations = Conversations()
    let store = MenuBarStore(
        bots: BotListStore(client: client, relay: relay), inbox: inbox
    ) { bot in
        if let made = conversations.made[bot.dmConversationID] { return made }
        let made = ConversationStore(
            client: client, relay: relay, conversationID: bot.dmConversationID, inbox: inbox)
        conversations.made[bot.dmConversationID] = made
        return made
    }
    await store.load()
    return Rig(store: store, conversations: conversations)
}

private func event(_ body: Hearth_V1_SubscribeResponse.OneOf_Event) -> Hearth_V1_SubscribeResponse {
    .with { $0.event = body }
}

private func closed(_ item: Hearth_V1_InboxItem) -> Hearth_V1_InboxItem {
    var item = item
    item.state = .closed
    return item
}

@MainActor
@Test func aWorkingBotAppearsInWorkingAndLeavesWhenItsRunEnds() async {
    let rig = await rig()
    #expect(rig.store.working.isEmpty)

    // The reply script: the run works, the reply streams, the run is done.
    let events = Fixtures.replyEvents
    await rig.deliver(Array(events.prefix(2)))
    #expect(rig.store.working.map(\.bot.id) == [Fixtures.bot.id])
    #expect(rig.store.working.first?.run.id == Fixtures.workingRun.id)
    #expect(rig.store.glyph == MenuBarStore.Glyph(working: true, needsYou: true))
    #expect(rig.store.spokenName == "Hearth, 1 working, 4 needs you")

    await rig.deliver(Array(events.dropFirst(2)))
    #expect(rig.store.working.isEmpty)
    #expect(!rig.store.glyph.working)
}

@MainActor
@Test func aFailedRunLandsInNeedsYouAsItsFailureItemOnce() async {
    let rig = await rig()
    await rig.deliver([
        event(.inboxChanged(.with { $0.item = closed(Fixtures.approval) })),
        event(.inboxChanged(.with { $0.item = closed(Fixtures.question) })),
        event(.inboxChanged(.with { $0.item = closed(Fixtures.widgetQuestion) })),
        event(.inboxChanged(.with { $0.item = closed(Fixtures.failure) })),
    ])
    #expect(rig.store.waiting.isEmpty)
    #expect(rig.store.needsCount == 0)
    #expect(rig.store.glyph == MenuBarStore.Glyph(working: false, needsYou: false))
    #expect(rig.store.spokenName == "Hearth")

    // The run fails, and the core opens its failure item.
    await rig.deliver([
        event(.runChanged(.with { $0.run = Fixtures.failedRun })),
        event(.inboxChanged(.with { $0.item = Fixtures.failure })),
    ])
    #expect(rig.store.working.isEmpty)
    #expect(rig.store.waiting.map(\.item.id) == [Fixtures.failure.id])
    #expect(rig.store.waiting.first?.bot?.id == Fixtures.bot.id)
    #expect(rig.store.needsCount == 1)
    #expect(rig.store.glyph == MenuBarStore.Glyph(working: false, needsYou: true))
    #expect(rig.store.spokenName == "Hearth, 1 needs you")
}

@MainActor
@Test func aResyncFromTheRelayRebuildsWorkingAndNeedsYou() async throws {
    let relay = EventRelay(subscribe: { _ in
        AsyncThrowingStream { continuation in
            continuation.yield(
                .with {
                    $0.sequence = 9
                    $0.event = .resync(Hearth_V1_Resync())
                })
        }
    })
    // The core says Dana's latest run works and four items are open.
    let rig = await rig(client: PreviewClient(latestRun: Fixtures.workingRun), relay: relay)
    #expect(rig.store.working.map(\.bot.id) == [Fixtures.bot.id])
    // What the relay delivered since says otherwise, but some was lost.
    var done = Fixtures.workingRun
    done.state = .done
    await rig.deliver([
        event(.runChanged(.with { $0.run = done })),
        event(.inboxChanged(.with { $0.item = closed(Fixtures.approval) })),
    ])
    #expect(rig.store.working.isEmpty)
    #expect(rig.store.needsCount == 3)

    relay.start()
    for _ in 0..<400 where rig.store.needsCount != 4 || rig.store.working.isEmpty {
        try await Task.sleep(for: .milliseconds(5))
    }
    relay.stop()
    #expect(rig.store.working.map(\.bot.id) == [Fixtures.bot.id])
    #expect(rig.store.needsCount == 4)
    #expect(relay.lastSequence == 9)
}

@MainActor
@Test func theQuickAskGoesToTheChiefOfStaffUnlessTheOwnerChoseSomeone() async {
    let rig = await rig()
    #expect(rig.store.bot?.id == Fixtures.chiefOfStaff.id)
    rig.store.chosenBotID = "bot-gone"
    #expect(rig.store.bot?.id == Fixtures.chiefOfStaff.id)
    rig.store.pick(Fixtures.bot)
    #expect(rig.store.bot?.id == Fixtures.bot.id)
}

@MainActor
@Test func theQuickAskWaitsWhileTheBotsLatestRunWorks() async {
    let rig = await rig(client: PreviewClient(latestRun: Fixtures.workingRun))
    rig.store.pick(Fixtures.bot)
    rig.store.draft = "Are you there?"
    #expect(!rig.store.canAsk)
    #expect(await rig.store.ask() == false)
    rig.store.pick(Fixtures.chiefOfStaff)
    #expect(rig.store.canAsk)
}

@MainActor
@Test func theQuickAskSendsIntoTheBotsDMAndShowsTheNewestTwoMessages() async {
    let rig = await rig()
    rig.store.pick(Fixtures.bot)
    #expect(!rig.store.canAsk)
    rig.store.draft = "   "
    #expect(!rig.store.canAsk)
    #expect(await rig.store.ask() == false)

    rig.store.draft = "Add a section on the free plans."
    #expect(rig.store.canAsk)
    #expect(await rig.store.ask())
    #expect(rig.store.draft.isEmpty)
    #expect(rig.store.error == nil)
    #expect(rig.store.replyBot?.id == Fixtures.bot.id)
    #expect(rig.store.replyConversationID == Fixtures.bot.dmConversationID)
    #expect(rig.store.reply.map(\.content) == ["Add a section on the free plans."])
    // The run is queued, so a second ask to Dana waits for it.
    rig.store.draft = "And the noise?"
    #expect(!rig.store.canAsk)

    // The reply opens and streams; the reply area is the owner's message
    // and the reply, without the tool rows.
    await rig.deliver(Array(Fixtures.replyEvents.prefix(5)))
    let reply = rig.store.reply
    #expect(
        Set(reply.map(\.id)) == [Fixtures.sendResponse.message.id, Fixtures.streamingReply.id])
    let streamed = reply.first { $0.id == Fixtures.streamingReply.id }
    #expect(streamed?.content == Fixtures.replyDeltas.prefix(2).joined())
}
