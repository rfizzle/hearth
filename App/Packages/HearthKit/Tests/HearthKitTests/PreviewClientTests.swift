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
