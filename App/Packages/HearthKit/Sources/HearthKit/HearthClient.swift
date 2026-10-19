// HearthClient is the one seam between the app and the core: stores take it in
// their initialiser, LiveClient speaks to hearthd and PreviewClient serves
// fixtures to previews, the gallery and store tests.

/// What a store may ask of the core.
public protocol HearthClient: Sendable {
    /// The core's version string.
    func ping() async throws -> String
    /// Streams `count` words from the core; 0 asks for the core's default.
    func streamHello(count: Int32) -> AsyncThrowingStream<String, Error>
}
