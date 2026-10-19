// Placeholder test for HearthFeatures: the hello store keeps what it streamed.

import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
@Test func helloStoreKeepsTheStreamedWords() async {
    let store = HelloStore()
    await store.stream(from: PreviewClient(), count: 3)
    #expect(store.transcript == PreviewClient.words.prefix(3).joined(separator: " "))
    #expect(store.error == nil)
}
