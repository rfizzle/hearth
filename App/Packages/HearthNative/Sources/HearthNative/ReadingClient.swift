// ReadingClient is the app's side of the reading service: the app never
// parses a file the core names, it opens it and hands the open file to the
// sandboxed service over XPC, then waits for the reply at most the time it is
// given.
//
// One read runs at a time; a call that comes while one is running is answered
// busy. When no reply comes in time the service is killed and its connection
// dropped, and when the service dies or the connection breaks the connection
// is dropped; either way the next read starts a fresh service, so a file that
// hangs or crashes the reader costs that one read. What the service sends back
// is checked as if it came from anyone: a bounded size, only the pages asked,
// each page's text and labels capped, and a refusal passed on only when it is
// one of the extractor's own sentences.
// See docs/decisions.md#hearth--decisions

import Darwin
import Foundation
import HearthKit
import HearthReading
import SwiftProtobuf
import Synchronization

/// What a read came to.
public enum ReadOutcome: Sendable, Equatable {
    case pages(ExtractedPages)
    /// The file was refused, in one of the extractor's sentences.
    case refused(String)
    /// The service could not read it, or its reply was not one.
    case failed
    /// The service ended or the connection broke before it replied.
    case crashed
    /// No reply came in time; the service was killed.
    case late
    /// A read was already running; this one was not started.
    case busy
}

/// Reads the text of pages of a file the core names. ReadingClient is the
/// real one; a test hands a fake.
public protocol PageReading: Sendable {
    /// Reads `pages` (from 1; empty for the first pages) of the file at
    /// `path`, reading for at most `extractFor` and waiting at most `waitFor`.
    func read(path: String, pages: [Int], extractFor: Duration, waitFor: Duration) async
        -> ReadOutcome
    /// Drops the connection and ends a read that is running.
    func close()
}

public final class ReadingClient: PageReading {
    /// The largest reply taken from the service: ten pages of 1 MB of text,
    /// with room for their labels.
    public static let maxReply = 12 * 1024 * 1024
    /// The longest label kept.
    static let maxLabel = 64

    private let connect: @Sendable () -> NSXPCConnection
    private let state = Mutex(State())

    private struct State {
        var link: Link?
        var running = false
    }

    /// A client that reaches the service through the connections `connect`
    /// makes; a test connects to a listener of its own.
    public init(connect: @escaping @Sendable () -> NSXPCConnection) {
        self.connect = connect
    }

    /// A client of the reading service bundled in the app.
    public static func service() -> ReadingClient {
        ReadingClient { NSXPCConnection(serviceName: Reading.serviceName) }
    }

    deinit { state.withLock { $0.link }?.connection.invalidate() }

    public func read(
        path: String, pages: [Int], extractFor: Duration, waitFor: Duration
    ) async -> ReadOutcome {
        let file: FileHandle
        do {
            file = try TextExtractor.open(path)
        } catch let refusal as ExtractRefusal {
            return .refused(refusal.message)
        } catch {
            return .failed
        }
        var request = Hearth_V1_ExtractTextRequest()
        request.pages = pages.map { Int32(clamping: $0) }
        guard let body = try? request.serializedData() else { return .failed }
        guard let link = begin() else { return .busy }
        let seconds = Double(extractFor.components.seconds)
        let (outcome, drop) = await withCheckedContinuation { continuation in
            let once = Once(continuation)
            let proxy = link.connection.remoteObjectProxyWithErrorHandler { _ in
                once.resume((.crashed, true))
            }
            guard let reader = proxy as? ReadingService else { return once.resume((.failed, true)) }
            reader.read(file, request: body, seconds: seconds) { data, refusal in
                once.resume(Self.outcome(data, refusal, asked: pages))
            }
            DispatchQueue.global().asyncAfter(deadline: .now() + Self.interval(waitFor)) {
                once.resume((.late, true))
            }
        }
        finish(link, dropping: drop, kill: outcome == .late)
        return outcome
    }

    public func close() {
        let (link, running) = state.withLock { state in
            defer { state.link = nil }
            return (state.link, state.running)
        }
        guard let link else { return }
        if running { link.kill() }
        link.connection.invalidate()
    }

    /// The connection to read on, made when there is none, and the claim to
    /// read; nil when a read is already running.
    private func begin() -> Link? {
        state.withLock { state in
            guard !state.running else { return nil }
            state.running = true
            if let link = state.link { return link }
            let link = Link(connect())
            link.connection.remoteObjectInterface = Reading.interface()
            // The service died or the connection ended: the next read makes
            // a new one, which starts a new service.
            link.connection.interruptionHandler = { [weak self, weak link] in
                if let link { self?.forget(link) }
            }
            link.connection.invalidationHandler = { [weak self, weak link] in
                if let link { self?.forget(link) }
            }
            link.connection.resume()
            state.link = link
            return link
        }
    }

    private func finish(_ link: Link, dropping: Bool, kill: Bool) {
        state.withLock { state in
            state.running = false
            if dropping, state.link === link { state.link = nil }
        }
        guard dropping else { return }
        if kill { link.kill() }
        link.connection.invalidate()
    }

    private func forget(_ link: Link) {
        let current = state.withLock { state in
            defer { if state.link === link { state.link = nil } }
            return state.link === link
        }
        if current { link.connection.invalidate() }
    }

    /// The outcome of a reply and whether the connection should be dropped:
    /// a reply that is not one drops it.
    static func outcome(_ data: Data?, _ refusal: String?, asked: [Int]) -> (ReadOutcome, Bool) {
        if let refusal {
            guard data == nil, ExtractRefusal.sentences.contains(refusal) else {
                return (.failed, true)
            }
            return (.refused(refusal), false)
        }
        guard let data else { return (.failed, false) }
        guard data.count <= maxReply,
            let response = try? Hearth_V1_ExtractTextResponse(serializedBytes: data)
        else { return (.failed, true) }
        return (.pages(checked(response.pages, asked: asked)), false)
    }

    /// The pages the service sent that were asked for, each once, in a fresh
    /// message: text and labels capped, nothing else carried over.
    static func checked(_ pages: ExtractedPages, asked: [Int]) -> ExtractedPages {
        let wanted = Set(asked.isEmpty ? Array(1...TextExtractor.maxPages) : asked)
        var seen = Set<Int32>()
        var out: ExtractedPages = []
        for page in pages.prefix(TextExtractor.maxPages) {
            guard wanted.contains(Int(page.page)), seen.insert(page.page).inserted,
                page.source == .textLayer || page.source == .recognised
            else { continue }
            var clean = Hearth_V1_ExtractedPage()
            clean.page = page.page
            clean.source = page.source
            clean.text = TextExtractor.capped(page.text)
            clean.labels = page.labels.prefix(TextExtractor.maxLabels).map {
                String($0.unicodeScalars.prefix(maxLabel))
            }
            out.append(clean)
        }
        return out
    }

    private static func interval(_ duration: Duration) -> DispatchTimeInterval {
        let parts = duration.components
        return .nanoseconds(Int(parts.seconds * 1_000_000_000 + parts.attoseconds / 1_000_000_000))
    }
}

/// A connection to the service. The connection is used from any thread, as
/// XPC allows.
private final class Link: @unchecked Sendable {
    let connection: NSXPCConnection
    init(_ connection: NSXPCConnection) { self.connection = connection }

    /// Ends the service process behind the connection. A listener in this
    /// process, as in a test, is never killed.
    func kill() {
        let pid = connection.processIdentifier
        guard pid > 0, pid != getpid() else { return }
        Darwin.kill(pid, SIGKILL)
    }
}

/// Resumes a continuation with the first outcome only.
private final class Once: Sendable {
    private let continuation: Mutex<CheckedContinuation<(ReadOutcome, Bool), Never>?>
    init(_ continuation: CheckedContinuation<(ReadOutcome, Bool), Never>) {
        self.continuation = Mutex(continuation)
    }
    func resume(_ outcome: (ReadOutcome, Bool)) {
        continuation.withLock { $0.take() }?.resume(returning: outcome)
    }
}
