// PreviewClient answers from fixtures instead of the core, for previews, the
// gallery and store tests. The words are a static list until a second fixture
// earns a Fixtures folder.

/// A client that never leaves the process.
public struct PreviewClient: HearthClient {
    /// The words streamHello cycles through.
    public static let words = ["Hello", "from", "the", "preview", "client."]

    public init() {}

    public func ping() async throws -> String {
        "preview"
    }

    public func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        let total = count > 0 ? Int(count) : 10
        return AsyncThrowingStream { continuation in
            for index in 0..<total {
                continuation.yield(Self.words[index % Self.words.count])
            }
            continuation.finish()
        }
    }
}
