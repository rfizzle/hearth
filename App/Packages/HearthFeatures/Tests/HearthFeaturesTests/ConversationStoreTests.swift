// Store tests for a conversation: Dana's scripted reply replayed one event at
// a time, a resync through the relay, paging, send and stop, against
// PreviewClient's DM.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func store(
    pageSize: Int32 = 0, relay: EventRelay = EventRelay(client: PreviewClient())
) -> ConversationStore {
    ConversationStore(
        client: PreviewClient(), relay: relay, conversationID: Fixtures.dmConversation.id,
        pageSize: pageSize)
}

/// What the reply and the run look like after one event.
private struct After {
    let text: String
    let streaming: Bool
    let run: Hearth_V1_RunState?
}

@MainActor
@Test func theScriptedReplyStreamsIntoTheLastMessageAndEndsTheRun() async {
    let store = store()
    await store.load()
    let reply = Fixtures.streamingReply.id
    var replyText: String? { store.messages.first { $0.id == reply }?.content }
    var replyStreams: Bool? { store.messages.first { $0.id == reply }?.streaming }
    #expect(store.messages == Fixtures.dmMessages)
    #expect(replyText == Fixtures.replyDeltas[0])

    // What each event leaves: the reply's text, whether it streams, the run.
    let expected: [After] = [
        After(text: Fixtures.replyDeltas[0], streaming: true, run: nil),
        After(text: Fixtures.replyDeltas[0], streaming: true, run: .working),
        After(text: "", streaming: true, run: .working),
        After(text: Fixtures.replyDeltas[0], streaming: true, run: .working),
        After(text: Fixtures.replyDeltas[0...1].joined(), streaming: true, run: .working),
        After(text: Fixtures.replyDeltas.joined(), streaming: true, run: .working),
        After(text: Fixtures.replyDeltas.joined(), streaming: false, run: .working),
        After(text: Fixtures.replyDeltas.joined(), streaming: false, run: .done),
    ]
    #expect(expected.count == Fixtures.replyEvents.count)
    for (event, after) in zip(Fixtures.replyEvents, expected) {
        await store.handle(event)
        #expect(replyText == after.text, "after event \(event.sequence)")
        #expect(replyStreams == after.streaming, "after event \(event.sequence)")
        #expect(store.run?.state == after.run, "after event \(event.sequence)")
        #expect(store.isWorking == (after.run == .working), "after event \(event.sequence)")
        #expect(store.messages.count == Fixtures.dmMessages.count)
    }
}

@MainActor
@Test func aResyncFromTheRelayRefetchesThePage() async throws {
    let relay = EventRelay(subscribe: { _ in
        AsyncThrowingStream { continuation in
            continuation.yield(
                .with {
                    $0.sequence = 9
                    $0.event = .resync(Hearth_V1_Resync())
                })
        }
    })
    let store = store(relay: relay)
    await store.load()
    var stray = Fixtures.sendResponse.message
    stray.content = "Not in the core's page"
    await store.handle(.with { $0.event = .messageUpserted(.with { $0.message = stray }) })
    #expect(store.messages.count == Fixtures.dmMessages.count + 1)

    relay.start()
    for _ in 0..<400 where store.messages.count != Fixtures.dmMessages.count {
        try await Task.sleep(for: .milliseconds(5))
    }
    relay.stop()
    #expect(store.messages == Fixtures.dmMessages)
    #expect(relay.lastSequence == 9)
}

@MainActor
@Test func olderPagesArePrependedUntilNoneAreLeft() async {
    let store = store(pageSize: 4)
    await store.load()
    #expect(store.messages.map(\.seq) == [7, 8, 9, 10])
    #expect(store.hasOlder)
    await store.loadOlder()
    #expect(store.messages.map(\.seq) == Array(3...10))
    await store.loadOlder()
    #expect(store.messages.map(\.seq) == Array(1...10))
    #expect(!store.hasOlder)
    await store.loadOlder()
    #expect(store.messages.count == 10)
}

@MainActor
@Test func eventsForAnotherConversationAreIgnored() async {
    let store = store()
    await store.load()
    var elsewhere = Fixtures.workingRun
    elsewhere.conversationID = "conversation-other"
    await store.handle(.with { $0.event = .runChanged(.with { $0.run = elsewhere }) })
    await store.handle(
        .with {
            $0.event = .messageDelta(
                .with {
                    $0.conversationID = "conversation-other"
                    $0.messageID = Fixtures.streamingReply.id
                    $0.text = "!"
                })
        })
    #expect(store.run == nil)
    #expect(store.messages == Fixtures.dmMessages)
}

@MainActor
@Test func sendAppendsTheOwnersMessageAndHoldsSendWhileTheRunWorks() async {
    let store = store()
    await store.load()
    #expect(!store.isWorking)
    await store.send("Add a section on running costs.")
    #expect(store.messages.last == Fixtures.sendResponse.message)
    #expect(store.run?.id == Fixtures.sendResponse.run.id)
    #expect(store.isWorking)
    await store.cancel()
    #expect(store.error == nil)
}

@MainActor
@Test func eachToolCallCarriesItsResult() async {
    let store = store()
    await store.load()
    let tools = store.rows.compactMap { row -> (String, Hearth_V1_Message?)? in
        guard case .tool(let call, _, let result) = row else { return nil }
        return (call.callID, result)
    }
    #expect(tools.map(\.0) == ["call-1", "call-2", "call-3"])
    #expect(tools.allSatisfy { $0.1 != nil })
    #expect(tools.last?.1?.files == [Fixtures.briefFile])
    #expect(store.rows.count == Fixtures.dmMessages.count - 3)
}

@MainActor
@Test func aResultWhoseCallIsOnAnOlderPageStandsAlone() async {
    let store = store(pageSize: 2)
    await store.load()
    #expect(store.messages.map(\.seq) == [9, 10])
    guard case .tool(let call, _, let result) = store.rows.first else {
        Issue.record("the first row is not a tool row")
        return
    }
    #expect(call.callID == "call-3")
    #expect(result?.seq == 9)
}

@MainActor
@Test func aRunEventThatBeatsTheSendAnswerIsKept() async {
    let store = store()
    var done = Fixtures.sendResponse.run
    done.state = .done
    await store.handle(.with { $0.event = .runChanged(.with { $0.run = done }) })
    await store.send("Add a section on running costs.")
    #expect(store.run?.state == .done)
    #expect(!store.isWorking)
}

/// Where the conversation is after one event of the park script.
private struct Parked {
    let run: Hearth_V1_RunState
    let waiting: Bool
    let card: Bool
}

@MainActor
@Test func aParkedRunWaitsWithItsCardAndResumesOnceAnswered() async {
    let relay = EventRelay(client: PreviewClient())
    let inbox = InboxStore(client: PreviewClient(), relay: relay)
    let store = ConversationStore(
        client: PreviewClient(), relay: relay, conversationID: Fixtures.dmConversation.id,
        inbox: inbox)
    await store.load()
    await store.handle(.with { $0.event = .runChanged(.with { $0.run = Fixtures.workingRun }) })
    #expect(store.isWorking)
    #expect(!store.isWaiting)
    #expect(store.waitingPrompt(botName: Fixtures.bot.name) == nil)

    var card: Hearth_V1_InboxItem? {
        store.rows.compactMap { row -> Hearth_V1_InboxItem? in
            guard case .request(_, let item) = row else { return nil }
            return item
        }.first
    }
    let expected: [Parked] = [
        Parked(run: .waitingOwner, waiting: true, card: false),
        Parked(run: .waitingOwner, waiting: true, card: false),
        Parked(run: .waitingOwner, waiting: true, card: true),
        Parked(run: .waitingOwner, waiting: true, card: false),
        Parked(run: .waitingOwner, waiting: true, card: false),
        Parked(run: .waitingOwner, waiting: true, card: false),
        Parked(run: .working, waiting: false, card: false),
        Parked(run: .done, waiting: false, card: false),
    ]
    #expect(expected.count == Fixtures.parkEvents.count)
    for (event, after) in zip(Fixtures.parkEvents, expected) {
        await inbox.handle(event)
        await store.handle(event)
        let step = "after event \(event.sequence)"
        #expect(store.run?.state == after.run, "\(step)")
        #expect(store.isWaiting == after.waiting, "\(step)")
        #expect((card?.id == Fixtures.approval.id) == after.card, "\(step)")
        #expect(
            store.waitingPrompt(botName: Fixtures.bot.name)
                == (after.waiting ? "Answer Dana above first" : nil), "\(step)")
    }
    // The request stays in the thread as a line, the decision as a notice.
    let kinds = store.messages.map(\.kind)
    #expect(kinds.contains(.approvalRequest))
    #expect(kinds.contains(.approvalDecision))
    #expect(store.messages.count == Fixtures.dmMessages.count + 3)
}

@MainActor
@Test func anOpenItemHoldsTheConversationEvenWhenTheRunIsForgotten() async {
    let relay = EventRelay(client: PreviewClient())
    let inbox = InboxStore(client: PreviewClient(), relay: relay)
    await inbox.load()
    let store = ConversationStore(
        client: PreviewClient(), relay: relay, conversationID: Fixtures.dmConversation.id,
        inbox: inbox)
    #expect(store.run == nil)
    #expect(store.waitingItem?.id == Fixtures.approval.id)
    #expect(store.isWaiting)
    let elsewhere = ConversationStore(
        client: PreviewClient(), relay: relay, conversationID: "conversation-other", inbox: inbox)
    #expect(!elsewhere.isWaiting)
}

@MainActor
@Test func aResyncForgetsTheRunUntilTheCoreReportsItAgain() async {
    let store = store()
    await store.handle(.with { $0.event = .runChanged(.with { $0.run = Fixtures.workingRun }) })
    #expect(store.isWorking)
    await store.handle(.with { $0.event = .resync(Hearth_V1_Resync()) })
    #expect(store.run == nil)
    #expect(store.messages == Fixtures.dmMessages)
}

@MainActor
@Test func aScreenOpenedMidRunShowsTheRunWithoutAnEvent() async {
    let store = ConversationStore(
        client: PreviewClient(latestRun: Fixtures.workingRun),
        relay: EventRelay(client: PreviewClient()), conversationID: Fixtures.dmConversation.id)
    #expect(!store.isWorking)
    await store.load()
    #expect(store.run == Fixtures.workingRun)
    #expect(store.isWorking)
    #expect(!store.canRetry)
}

@MainActor
@Test func retryAfterAFailedRunAppliesTheNewRun() async {
    let store = store()
    await store.load()
    await store.retry()
    #expect(store.run == nil)

    await store.handle(.with { $0.event = .runChanged(.with { $0.run = Fixtures.failedRun }) })
    #expect(store.canRetry)
    await store.retry()
    #expect(store.run?.id == "run-retry")
    #expect(store.run?.state == .queued)
    #expect(store.isWorking)
    #expect(!store.canRetry)
}

@MainActor
@Test func aStoppedRunCanBeRetried() async {
    var stopped = Fixtures.workingRun
    stopped.state = .cancelled
    let store = store()
    await store.handle(.with { $0.event = .runChanged(.with { $0.run = stopped }) })
    #expect(store.canRetry)
}

@MainActor
@Test func aDeletedMessageLeavesTheList() async {
    let store = store()
    await store.load()
    let doomed = Fixtures.streamingReply.id
    await store.handle(
        .with {
            $0.event = .messageDeleted(
                .with {
                    $0.conversationID = "conversation-other"
                    $0.messageID = doomed
                })
        })
    #expect(store.messages.contains { $0.id == doomed })
    await store.handle(
        .with {
            $0.event = .messageDeleted(
                .with {
                    $0.conversationID = Fixtures.dmConversation.id
                    $0.messageID = doomed
                })
        })
    #expect(!store.messages.contains { $0.id == doomed })
    #expect(store.messages.count == Fixtures.dmMessages.count - 1)
    #expect(!store.rows.contains { $0.id == doomed })
}
