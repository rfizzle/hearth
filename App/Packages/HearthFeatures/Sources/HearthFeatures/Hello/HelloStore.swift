// HelloStore holds the words the core has streamed so far. It caches and
// appends; what the words are and how many is the core's business.

import HearthKit
import Observation

/// The store behind the hello screen.
@MainActor
@Observable
public final class HelloStore {
    /// The words streamed so far, separated by spaces.
    public private(set) var transcript = ""
    /// The last stream's failure, if it failed.
    public private(set) var error: String?

    public init() {}

    /// Replaces the transcript with a fresh stream of `count` words from `client`.
    public func stream(from client: any HearthClient, count: Int32) async {
        transcript = ""
        error = nil
        do {
            for try await word in client.streamHello(count: count) {
                transcript += transcript.isEmpty ? word : " " + word
            }
        } catch {
            self.error = error.localizedDescription
        }
    }
}
