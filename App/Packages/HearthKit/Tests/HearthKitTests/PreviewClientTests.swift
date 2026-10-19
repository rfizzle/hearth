// Placeholder test for HearthKit: the preview client streams its fixture words.

import Testing

@testable import HearthKit

@Test func previewClientStreamsTheRequestedCount() async throws {
    var words: [String] = []
    for try await word in PreviewClient().streamHello(count: 3) {
        words.append(word)
    }
    #expect(words == Array(PreviewClient.words.prefix(3)))
}
