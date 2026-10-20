// PreviewClient serves Fixtures the way the core would: hello words, pages of
// the DM, and the scripted reply after a sequence.

import Testing

@testable import HearthKit

@Test func previewClientStreamsTheRequestedCount() async throws {
    var words: [String] = []
    for try await word in PreviewClient().streamHello(count: 3) {
        words.append(word)
    }
    #expect(words == Array(PreviewClient.words.prefix(3)))
}

@Test func listMessagesPagesBackwardsFromTheNewest() async throws {
    let client = PreviewClient()
    let newest = try await client.listMessages(
        conversationID: Fixtures.dmConversation.id, beforeSeq: 0, limit: 3)
    #expect(newest.messages.map(\.seq) == [8, 9, 10])
    #expect(newest.hasMore_p)

    let oldest = try await client.listMessages(
        conversationID: Fixtures.dmConversation.id, beforeSeq: 8, limit: 0)
    #expect(oldest.messages.map(\.seq) == Array(1...7))
    #expect(!oldest.hasMore_p)

    let other = try await client.listMessages(conversationID: "elsewhere", beforeSeq: 0, limit: 0)
    #expect(other.messages.isEmpty)
}

@Test func subscribeReplaysOnlyEventsAfterTheSequence() async throws {
    var sequences: [UInt64] = []
    for try await event in PreviewClient().subscribe(after: 5) {
        sequences.append(event.sequence)
    }
    let total = UInt64(Fixtures.replyEvents.count)
    #expect(sequences == Array(6...total))
}

@Test func listConversationsFiltersByBot() async throws {
    let client = PreviewClient()
    #expect(try await client.listConversations(botID: "").map(\.id) == [Fixtures.dmConversation.id])
    #expect(try await client.listConversations(botID: Fixtures.bot.id).count == 1)
    #expect(try await client.listConversations(botID: "bot-other").isEmpty)
}

@Test func draftBotServesTheFixtureDraft() async throws {
    let request = Hearth_V1_DraftBotRequest.with { $0.description_p = "Read up on competitors" }
    let draft = try await PreviewClient().draftBot(request)
    #expect(draft == Fixtures.draft)
    #expect(!draft.jobDescription.isEmpty && !draft.workingStyle.isEmpty)
}

@Test func signInAnswersWithoutTheCredential() async throws {
    let client = PreviewClient()
    let account = try await client.setAPIKey(provider: .anthropic, key: "sk-secret-1234")
    #expect(account.state == .signedIn)
    #expect(account.label == "1234")
    #expect(try await client.startChatGPTLogin() == Fixtures.authorizeURL)
}

@Test func anIntegrationKeyIsReportedByItsLastFour() async throws {
    let client = PreviewClient()
    let listed = try await client.listIntegrations()
    #expect(listed.map(\.integration) == [.braveSearch])
    #expect(listed.allSatisfy { !$0.configured && !$0.helpURL.isEmpty })

    let account = try await client.setIntegrationKey(.braveSearch, key: "BSA-secret-9876")
    #expect(account.configured)
    #expect(account.kind == .webSearch)
    #expect(account.label == "9876")
}

@Test func anAddedServiceIsItsOwnAccountWithoutTheKey() async throws {
    let client = PreviewClient()
    let known = try await client.listKnownServices()
    #expect(known.filter { $0.kind == .builtin }.count == 4)
    #expect(known.allSatisfy { !$0.baseURL.isEmpty && !$0.helpURL.isEmpty })

    let request = Hearth_V1_SetAPIKeyRequest.with {
        $0.displayName = "Acme gateway"
        $0.baseURL = "https://llm.acme.example/v1"
        $0.defaultModel = "acme-large"
        $0.key = "gw-secret-5678"
    }
    let account = try await client.saveService(request)
    #expect(account.provider == .openaiCompatible)
    #expect(!account.profile.isEmpty)
    #expect(account.displayName == "Acme gateway")
    #expect(account.label == "5678")
}

@Test func listInboxServesTheFixtureItemsAndTheDecisionsCloseThem() async throws {
    let client = PreviewClient()
    #expect(try await client.listInbox(includeClosed: false) == Fixtures.inbox)

    let decided = try await client.decide(itemID: Fixtures.approval.id, .deny, reason: "")
    #expect(decided.item.state == .closed && decided.item.decision == .deny)
    #expect(decided.item.outcome == "Denied")

    let answered = try await client.answer(itemID: Fixtures.question.id, "Both kinds")
    #expect(answered.item.state == .closed && answered.item.answer == "Both kinds")

    let acknowledged = try await client.acknowledge(itemID: Fixtures.failure.id)
    #expect(acknowledged.state == .closed)
}

@Test func listGrantsServesTheFixtureGrantsAndSetGrantsEchoesThem() async throws {
    let client = PreviewClient()
    #expect(try await client.listGrants(botID: Fixtures.bot.id) == Fixtures.grants)
    #expect(try await client.setGrants(botID: Fixtures.bot.id, []) == [])
}

@Test func theCurrentRunAndRetryAreServed() async throws {
    let client = PreviewClient()
    let conversation = try await client.getConversation(id: Fixtures.dmConversation.id)
    #expect(conversation.id == Fixtures.dmConversation.id)
    let run = try await client.retry(runID: Fixtures.failedRun.id)
    #expect(run.state == .queued && run.failure.isEmpty)
}

@Test func sendTakesAttachmentsAndDefaultsToNone() async throws {
    let client = PreviewClient()
    let plain = try await client.send(conversationID: "c", text: "Hello")
    let attached = try await client.send(
        conversationID: "c", text: "Hello", attachmentPaths: ["/tmp/a.pdf"])
    #expect(plain.message.content == "Hello" && attached.message.content == "Hello")
}
