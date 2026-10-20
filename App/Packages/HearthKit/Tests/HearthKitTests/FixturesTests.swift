// The fixtures hold the shapes later screens are drawn against: ten DM
// messages covering every row kind the slice shows, and a scripted reply in
// the order the core sends one.

import Testing

@testable import HearthKit

@Test func theOrgHasThreeTemplatesAndOneBot() {
    #expect(Fixtures.templates.count == 3)
    #expect(Fixtures.bot.dmConversationID == Fixtures.dmConversation.id)
    #expect(Fixtures.bot.folderPath.hasPrefix(Fixtures.org.workspacePath))
}

@Test func theDMCoversEveryRowTheTranscriptDraws() {
    let messages = Fixtures.dmMessages
    #expect(messages.count == 10)
    #expect(messages.map(\.seq) == Array(1...10))
    #expect(messages.contains { $0.authorBotID.isEmpty && $0.kind == .text })
    #expect(messages.contains { $0.kind == .system })
    #expect(messages.contains { $0.kind == .toolCall && !$0.tool.summary.isEmpty })
    #expect(messages.contains { $0.kind == .toolResult && !$0.files.isEmpty })
    let searches = messages.filter { $0.tool.name == "web.search" }.map(\.kind)
    #expect(searches == [.toolCall, .toolResult])
    #expect(messages.last?.streaming == true)
}

@Test func theReplyEventsFollowTheCoresOrder() {
    let events = Fixtures.replyEvents.compactMap(\.event)
    #expect(Fixtures.replyEvents.map(\.sequence) == Array(1...UInt64(events.count)))

    guard case .runChanged(let started) = events[1], case .messageUpserted(let opened) = events[2],
        case .messageUpserted(let finished) = events[events.count - 2],
        case .runChanged(let ended) = events[events.count - 1]
    else {
        Issue.record("events out of order: \(events)")
        return
    }
    #expect(started.run.state == .working)
    #expect(opened.message.streaming && opened.message.content.isEmpty)
    #expect(!finished.message.streaming)
    #expect(finished.message.content == Fixtures.replyDeltas.joined())
    #expect(ended.run.state == .done)

    let deltas = events.compactMap { event -> String? in
        if case .messageDelta(let delta) = event { return delta.text }
        return nil
    }
    #expect(deltas == Fixtures.replyDeltas)
}

@Test func theInboxHoldsAnApprovalTwoQuestionsAndAFailure() {
    let items = Fixtures.inbox
    #expect(items.map(\.kind) == [.approval, .question, .question, .failure])
    #expect(Fixtures.widgetQuestion.widget.singleChoice.options.count == 3)
    #expect(Fixtures.widgetQuestionMessage.widget == Fixtures.widgetQuestion.widget)
    #expect(Fixtures.widgetQuestionMessage.id == Fixtures.widgetQuestion.messageID)
    #expect(items.allSatisfy { $0.state == .open && !$0.title.isEmpty })
    #expect(Fixtures.approval.call.name == "files.delete")
    #expect(Fixtures.approval.effect.contains("brief.md"))
    #expect(Fixtures.question.suggestedAnswers.count == 2)
    #expect(Fixtures.failure.effect == Fixtures.failedRun.failure)
}

@Test func theGrantsAreTheFiveCapabilitiesWithTheOwnFolderMarked() {
    let grants = Fixtures.grants
    #expect(
        grants.map(\.capability)
            == [.filesRead, .filesWrite, .filesDelete, .webFetch, .webSearch])
    #expect(
        grants.filter(\.ownFolder).map(\.folder)
            == Array(repeating: Fixtures.bot.folderPath, count: 3))
    #expect(grants.first { $0.capability == .filesDelete }?.canAllow == false)
}

@Test func theParkEventsWaitThenResumeInTheCoresOrder() {
    let events = Fixtures.parkEvents.compactMap(\.event)
    #expect(Fixtures.parkEvents.map(\.sequence) == Array(1...UInt64(events.count)))

    guard case .runChanged(let waiting) = events[0], case .messageUpserted(let request) = events[1],
        case .inboxChanged(let opened) = events[2], case .inboxChanged(let closed) = events[3],
        case .messageUpserted(let decision) = events[4],
        case .messageUpserted(let result) = events[5], case .runChanged(let resumed) = events[6]
    else {
        Issue.record("events out of order: \(events)")
        return
    }
    #expect(waiting.run.state == .waitingOwner)
    #expect(request.message.kind == .approvalRequest)
    #expect(opened.item.state == .open && opened.item.messageID == request.message.id)
    #expect(closed.item.state == .closed && closed.item.decision == .allowOnce)
    #expect(decision.message.kind == .approvalDecision)
    #expect(result.message.kind == .toolResult)
    #expect(resumed.run.state == .working)
}
