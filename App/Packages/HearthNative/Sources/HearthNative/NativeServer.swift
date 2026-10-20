// NativeServer is the second listener in the app: the core calls back into it
// for what only Apple's frameworks do, today reading the text of picture
// pages and images. It listens on 127.0.0.1 at a port the system picks, takes
// connections from loopback only, and serves them one at a time, one request
// each. Every request must carry the native token, a token the app makes for
// this direction alone and gives the helper on stdin after the launch token,
// as a bearer, checked in constant time before the method or the route.
//
// The bounds, here, in NativeHTTP, NativeService and ReadingClient: 8 KB of
// headers and 64 KB of body; 5 s for a request to arrive and 40 s for a
// connection in all; ten pages a call; reading done only in the sandboxed
// reading service, one read at a time, waited for at most 28 s before the
// service is killed; a reply of at most 4 MB; four connections waiting, and
// any more closed unanswered. Nothing is written; the log has a file's name
// and how long it took, never its text.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthKit
import Network
import Synchronization
import os

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
    private let reader: any PageReading
    private let onFailure: @Sendable () -> Void

    /// A server that answers requests carrying `token`, the native token,
    /// reading files through `reader`. It does not listen until start.
    /// `onFailure` is told when the listener fails after it was ready, so the
    /// caller can stop handing out its port.
    public init(
        token: String, reader: any PageReading,
        onFailure: @escaping @Sendable () -> Void = {}
    ) throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: .ipv4(.loopback), port: .any)
        parameters.acceptLocalOnly = true
        listener = try NWListener(using: parameters)
        service = NativeService(token: token, reader: reader)
        self.reader = reader
        self.onFailure = onFailure
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
            let ready = Flag()
            listener.stateUpdateHandler = { [weak listener, onFailure] state in
                switch state {
                case .ready:
                    // A listener with no port is no listener.
                    guard let port = listener?.port?.rawValue, port > 0 else {
                        return once.resume(.failure(NWError.posix(.EADDRNOTAVAIL)))
                    }
                    ready.value.withLock { $0 = true }
                    once.resume(.success(port))
                // A listener that cannot bind now is treated as one that
                // failed, so the helper is launched without it, not never.
                // One that fails after it was ready is reported.
                case .failed(let error), .waiting(let error):
                    if ready.value.withLock({ $0 }) { onFailure() }
                    once.resume(.failure(error))
                case .cancelled: once.resume(.failure(CancellationError()))
                default: break
                }
            }
            listener.start(queue: queue)
        }
    }

    /// Stops listening, closes every connection waiting and the one being
    /// served, and ends its reading with the reading service's connection.
    public func stop() {
        listener.newConnectionHandler = nil
        listener.cancel()
        arrivals.finish()
        worker.withLock { $0?.cancel() }
        active.slot.withLock { $0?.cancel() }
        reader.close()
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

/// A flag a listener's state handler sets and reads.
private final class Flag: Sendable {
    let value = Mutex(false)
}
