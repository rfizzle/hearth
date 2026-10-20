// NativeService is the native server's answers over complete requests,
// without the network: the token check, the two routes, the JSON, the time a
// read may take and the reply's size.
//
// A read runs on a queue of its own, not on the server's serial loop. The
// call waits for it at most replyTime (28 s, inside the core's 30 s) and then
// answers without it, leaving the read to finish by itself; the extractor
// stops at the next page after extractTime (25 s), but a page that never
// returns is never interrupted. While such an abandoned read is still running
// no other is started: the call is answered busy at once, which the core
// takes as the app not being there for the rest of its run. So at most one
// read is ever alive, and the serial loop is never held by one.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthKit
import SwiftProtobuf
import Synchronization
import os

/// The pages an extractor read, and the instant it must stop reading by.
public typealias ExtractedPages = [Hearth_V1_ExtractedPage]
public typealias Deadline = ContinuousClock.Instant

/// Reads the text of pages of a file the core names. TextExtractor is the
/// real one; a test hands a fake.
public protocol TextExtracting: Sendable {
    /// The pages of the file at `path` that could be read before `deadline`,
    /// in the order asked; `pages` are counted from 1, empty for the first
    /// pages of the file.
    func extract(path: String, pages: [Int], until deadline: Deadline) throws -> ExtractedPages
}

/// A path or file the extractor will not read, said in a fixed sentence.
public struct ExtractRefusal: Error, Sendable, Equatable {
    public let message: String
    public init(_ message: String) { self.message = message }
}

public struct NativeService: Sendable {
    /// Bumped when the native surface changes.
    public static let version = Int32(HearthNative.version)
    /// The most pages one call reads.
    public static let maxPages = 10
    /// The time the extractor reads for; it stops at the next page after it.
    public static let extractTime: Duration = .seconds(25)
    /// The time a call waits for the read before it answers without it.
    public static let replyTime: Duration = .seconds(28)
    /// The largest reply; pages are dropped from the end until it fits.
    public static let maxReply = 4 * 1024 * 1024

    let token: String
    let extractor: any TextExtracting
    private let worker: ReadWorker
    private let log = Logger(subsystem: "com.rfizzle.hearth", category: "native")

    /// A service that answers requests carrying `token` with `extractor`,
    /// waiting `replyAfter` for a read; a test shortens it.
    public init(token: String, extractor: any TextExtracting, replyAfter: Duration = replyTime) {
        self.token = token
        self.extractor = extractor
        worker = ReadWorker(limit: replyAfter)
    }

    /// The bytes to send back for what was received.
    public func respond(to parsed: NativeParse) async -> Data {
        switch parsed {
        case .incomplete: return NativeHTTP.reply(.malformed)
        case .refused(let error): return NativeHTTP.reply(error)
        case .request(let request):
            switch NativeHTTP.route(request, token: token) {
            case .failure(let error): return NativeHTTP.reply(error)
            case .success(.ping):
                var ping = Hearth_V1_NativeServicePingResponse()
                ping.version = Self.version
                return encoded(ping)
            case .success(.extractText): return await extract(request.body)
            }
        }
    }

    private func extract(_ body: Data) async -> Data {
        guard let request = try? Hearth_V1_ExtractTextRequest(jsonUTF8Data: body),
            request.pages.count <= Self.maxPages,
            request.pages.allSatisfy({ (1...1_048_576).contains($0) })
        else { return NativeHTTP.reply(.malformed) }
        let started = ContinuousClock.now
        let name = URL(filePath: request.path).lastPathComponent
        let extractor = extractor
        let outcome = await worker.run {
            Result {
                try extractor.extract(
                    path: request.path, pages: request.pages.map(Int.init),
                    until: started + Self.extractTime)
            }
        }
        switch outcome {
        case .busy:
            log.info("refused \(name): busy with an earlier read")
            return NativeHTTP.reply(.busy)
        case .late:
            log.error("gave up waiting for \(name) after \(started.duration(to: .now))")
            return NativeHTTP.reply(.late)
        case .done(.success(let pages)):
            log.info("read \(pages.count) pages of \(name) in \(started.duration(to: .now))")
            return fitted(pages)
        case .done(.failure(let refusal as ExtractRefusal)):
            log.info("refused \(name): \(refusal.message, privacy: .public)")
            return NativeHTTP.reply(
                NativeHTTPError(status: 400, code: "invalid_argument", message: refusal.message))
        case .done(.failure):
            log.error("could not read \(name)")
            return NativeHTTP.reply(.failed)
        }
    }

    /// The reply for `pages`, dropping pages from the end until it is at most
    /// maxReply bytes. A page dropped is a page the core reports unread.
    private func fitted(_ pages: ExtractedPages) -> Data {
        var response = Hearth_V1_ExtractTextResponse()
        response.pages = pages
        while true {
            let data = encoded(response)
            if data.count <= Self.maxReply || response.pages.isEmpty { return data }
            response.pages.removeLast()
        }
    }

    private func encoded(_ message: some SwiftProtobuf.Message) -> Data {
        guard let body = try? message.jsonUTF8Data() else { return NativeHTTP.reply(.failed) }
        return NativeHTTP.reply(body)
    }
}

/// Runs one read at a time on a queue of its own and waits for it at most
/// `limit`. A read it stops waiting for keeps the worker busy until it ends.
final class ReadWorker: Sendable {
    enum Outcome: Sendable {
        /// An earlier read is still running; this one was not started.
        case busy
        /// The read did not end within the limit and was left to run.
        case late
        case done(Result<ExtractedPages, any Error>)
    }

    private let limit: Duration
    private let queue = DispatchQueue(label: "com.rfizzle.hearth.native.read")
    private let running = Mutex(false)

    init(limit: Duration) { self.limit = limit }

    func run(_ read: @escaping @Sendable () -> Result<ExtractedPages, any Error>) async -> Outcome {
        let started = running.withLock { busy in
            if busy { return false }
            busy = true
            return true
        }
        guard started else { return .busy }
        let parts = limit.components
        let wait = parts.seconds * 1_000_000_000 + parts.attoseconds / 1_000_000_000
        return await withCheckedContinuation { continuation in
            let once = Once(continuation)
            queue.async {
                let result = read()
                self.running.withLock { $0 = false }
                once.resume(.done(result))
            }
            DispatchQueue.global().asyncAfter(deadline: .now() + .nanoseconds(Int(wait))) {
                once.resume(.late)
            }
        }
    }

    /// Resumes a continuation with the first outcome only.
    private final class Once: Sendable {
        private let continuation: Mutex<CheckedContinuation<Outcome, Never>?>
        init(_ continuation: CheckedContinuation<Outcome, Never>) {
            self.continuation = Mutex(continuation)
        }
        func resume(_ outcome: Outcome) {
            continuation.withLock { $0.take() }?.resume(returning: outcome)
        }
    }
}
