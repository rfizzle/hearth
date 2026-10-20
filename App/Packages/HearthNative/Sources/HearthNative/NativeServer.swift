// NativeServer is the second listener in the app: the core calls back into it
// for what only Apple's frameworks do, today reading the text of picture
// pages and images. It listens on 127.0.0.1 at a port the system picks, takes
// connections from loopback only, and serves them one at a time, one request
// each. Every request must carry the launch token the app gave the helper, as
// a bearer, checked in constant time before anything else is looked at.
//
// The bounds, all here or in NativeHTTP: 8 KB of headers and 64 KB of body;
// 5 s for a request to arrive and 40 s for a connection in all; ten pages and
// 30 s of reading a call; a reply of at most 4 MB; four connections waiting,
// and any more closed unanswered. Nothing is written; the log has a file's
// name and how long it took, never its text.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthKit
import Network
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

/// The native service over complete requests, without the network: the
/// token check, the routes, the JSON and the reply's size.
public struct NativeService: Sendable {
    /// Bumped when the native surface changes.
    public static let version = Int32(HearthNative.version)
    /// The most pages one call reads.
    public static let maxPages = 10
    /// The time one call may spend reading.
    public static let extractTime: Duration = .seconds(30)
    /// The largest reply; pages are dropped from the end until it fits.
    public static let maxReply = 4 * 1024 * 1024

    let token: String
    let extractor: any TextExtracting
    private let log = Logger(subsystem: "com.rfizzle.hearth", category: "native")

    public init(token: String, extractor: any TextExtracting) {
        self.token = token
        self.extractor = extractor
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
        let clock = ContinuousClock()
        let started = clock.now
        let name = URL(filePath: request.path).lastPathComponent
        do {
            let pages = try extractor.extract(
                path: request.path, pages: request.pages.map(Int.init),
                until: started + Self.extractTime)
            log.info("read \(pages.count) pages of \(name) in \(started.duration(to: clock.now))")
            return fitted(pages)
        } catch let refusal as ExtractRefusal {
            log.info("refused \(name): \(refusal.message, privacy: .public)")
            return NativeHTTP.reply(
                NativeHTTPError(status: 400, code: "invalid_argument", message: refusal.message))
        } catch {
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

/// The listener: loopback, an ephemeral port, connections served one at a
/// time through NativeService.
public final class NativeServer: Sendable {
    /// How long a request may take to arrive in full.
    public static let receiveTime: Duration = .seconds(5)
    /// How long a connection may stay open in all, reading included.
    public static let connectionTime: Duration = .seconds(40)
    /// Connections waiting their turn; one more is closed unanswered.
    public static let maxWaiting = 4

    private let listener: NWListener
    private let service: NativeService
    private let queue = DispatchQueue(label: "com.rfizzle.hearth.native")
    private let waiting: AsyncStream<NWConnection>
    private let arrivals: AsyncStream<NWConnection>.Continuation
    private let worker = Mutex<Task<Void, Never>?>(nil)
    private let active = ActiveConnection()

    /// A server that answers requests carrying `token` with `extractor`. It
    /// does not listen until start.
    public init(token: String, extractor: any TextExtracting) throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: .ipv4(.loopback), port: .any)
        parameters.acceptLocalOnly = true
        listener = try NWListener(using: parameters)
        service = NativeService(token: token, extractor: extractor)
        (waiting, arrivals) = AsyncStream.makeStream(
            of: NWConnection.self, bufferingPolicy: .bufferingOldest(Self.maxWaiting))
    }

    /// Starts listening and returns the port, once the system has bound it.
    public func start() async throws -> UInt16 {
        let task = Task { [waiting, service, queue, active] in
            for await connection in waiting {
                active.slot.withLock { $0 = connection }
                await Self.serve(connection, with: service, on: queue)
                active.slot.withLock { $0 = nil }
            }
        }
        worker.withLock { $0 = task }
        listener.newConnectionHandler = { [arrivals] connection in
            guard Self.isLoopback(connection.endpoint) else { return connection.cancel() }
            if case .dropped(let extra) = arrivals.yield(connection) { extra.cancel() }
        }
        return try await withCheckedThrowingContinuation { continuation in
            let once = ResumeOnce(continuation)
            listener.stateUpdateHandler = { [weak listener] state in
                switch state {
                case .ready: once.resume(.success(listener?.port?.rawValue ?? 0))
                // A listener that cannot bind now is treated as one that
                // failed, so the helper is launched without it, not never.
                case .failed(let error), .waiting(let error): once.resume(.failure(error))
                case .cancelled: once.resume(.failure(CancellationError()))
                default: break
                }
            }
            listener.start(queue: queue)
        }
    }

    /// Stops listening, closes every connection waiting and the one being
    /// served, and cancels its reading, which stops at the next page.
    public func stop() {
        listener.newConnectionHandler = nil
        listener.cancel()
        arrivals.finish()
        worker.withLock { $0?.cancel() }
        active.slot.withLock { $0?.cancel() }
    }

    static func isLoopback(_ endpoint: NWEndpoint) -> Bool {
        guard case .hostPort(let host, _) = endpoint else { return false }
        switch host {
        case .ipv4(let address): return address.isLoopback
        case .ipv6(let address): return address.isLoopback
        default: return false
        }
    }

    /// Reads one request from `connection` within receiveTime, answers it,
    /// and closes the connection; the whole within connectionTime.
    private static func serve(
        _ connection: NWConnection, with service: NativeService, on queue: DispatchQueue
    ) async {
        guard !Task.isCancelled else { return connection.cancel() }
        connection.start(queue: queue)
        let deadline = Task {
            try? await Task.sleep(for: connectionTime)
            connection.cancel()
        }
        let arrival = Task {
            try? await Task.sleep(for: receiveTime)
            if !Task.isCancelled { connection.cancel() }
        }
        defer {
            deadline.cancel()
            connection.cancel()
        }
        let parsed = await receive(connection)
        arrival.cancel()
        guard let parsed else { return }
        let reply = await service.respond(to: parsed)
        await withCheckedContinuation { (done: CheckedContinuation<Void, Never>) in
            connection.send(content: reply, completion: .contentProcessed { _ in done.resume() })
        }
    }

    /// The next bytes on `link` and whether it has ended; nil when it failed.
    private static func chunk(of link: NWConnection) async -> (Data, Bool)? {
        await withCheckedContinuation { done in
            let most = NativeHTTP.maxBody
            link.receive(minimumIncompleteLength: 1, maximumLength: most) { data, _, end, error in
                done.resume(returning: error == nil ? (data ?? Data(), end) : nil)
            }
        }
    }

    /// The request on `connection`, or nil when it closed or failed first.
    /// The buffer is bounded: parse refuses past NativeHTTP's sizes.
    private static func receive(_ connection: NWConnection) async -> NativeParse? {
        var buffer = Data()
        while true {
            guard let (data, complete) = await chunk(of: connection) else { return nil }
            buffer.append(data)
            let parsed = NativeHTTP.parse(buffer)
            if parsed != .incomplete { return parsed }
            if complete { return nil }
        }
    }
}

/// The connection being served, so stop can close it.
private final class ActiveConnection: Sendable {
    let slot = Mutex<NWConnection?>(nil)
}

/// Resumes a continuation once, whichever state comes first.
private final class ResumeOnce: Sendable {
    private let continuation: Mutex<CheckedContinuation<UInt16, any Error>?>

    init(_ continuation: CheckedContinuation<UInt16, any Error>) {
        self.continuation = Mutex(continuation)
    }

    func resume(_ result: Result<UInt16, any Error>) {
        continuation.withLock { $0.take() }?.resume(with: result)
    }
}
