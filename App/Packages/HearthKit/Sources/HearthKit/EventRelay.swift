// EventRelay is the app's one subscription to the core's event stream. It
// follows subscribe in a single task, hands every event to the handlers the
// stores registered, in order, and remembers the last sequence it saw. When
// the stream ends or fails it waits, doubling the wait up to a ceiling, and
// subscribes again after that sequence, so a dropped connection loses nothing
// the core still holds; when the core holds too little it answers with a
// resync, which reaches every handler as the sign to refetch. One relay rather
// than a stream per store keeps reconnecting in one place.
// See docs/architecture.md#3-the-contract-sketch
//
// A new helper starts its sequences again at 1, so the app target calls
// reset when the helper's generation changes: the sequence goes back to 0 and
// every handler is told to refetch, as on a resync.

import Foundation

/// The fan-out from the core's event stream to the stores.
@MainActor
public final class EventRelay {
    /// Opens the core's stream after a sequence; 0 asks for live events only.
    public typealias Subscribe =
        @Sendable (UInt64) -> AsyncThrowingStream<
            Hearth_V1_SubscribeResponse, Error
        >
    /// What a store registers: called with each event, one at a time.
    public typealias Handler = @MainActor (Hearth_V1_SubscribeResponse) async -> Void

    /// A registered handler, for unregistering it.
    public struct Registration: Hashable, Sendable {
        let id = UUID()
    }

    /// The sequence of the last event delivered; the next subscribe resumes
    /// after it.
    public private(set) var lastSequence: UInt64 = 0

    private var subscribe: Subscribe
    private var handlers: [(registration: Registration, handle: Handler)] = []
    private var task: Task<Void, Never>?
    private let initialDelay: Duration
    private let maxDelay: Duration

    /// A relay over `subscribe` that waits `initialDelay` before the first
    /// reconnect and at most `maxDelay` before any later one.
    public init(
        subscribe: @escaping Subscribe,
        initialDelay: Duration = .milliseconds(250),
        maxDelay: Duration = .seconds(15)
    ) {
        self.subscribe = subscribe
        self.initialDelay = initialDelay
        self.maxDelay = maxDelay
    }

    /// A relay over `client`'s subscribe.
    public convenience init(client: any HearthClient) {
        self.init(subscribe: Self.subscribe(to: client))
    }

    /// Whether the relay is following the stream.
    public var isRunning: Bool {
        task != nil
    }

    /// Adds `handler`; it sees every event delivered from now on.
    @discardableResult
    public func register(_ handler: @escaping Handler) -> Registration {
        let registration = Registration()
        handlers.append((registration, handler))
        return registration
    }

    /// Removes the handler `registration` names.
    public func unregister(_ registration: Registration) {
        handlers.removeAll { $0.registration == registration }
    }

    /// Starts following the stream after the last sequence; a running relay
    /// carries on.
    public func start() {
        guard task == nil else { return }
        task = Task { await self.follow(resyncFirst: false) }
    }

    /// Stops following the stream; the last sequence is kept for `start`.
    public func stop() {
        task?.cancel()
        task = nil
    }

    /// Starts again from sequence 0, over `client` when one is given, and
    /// tells every handler to refetch: a new helper numbers its events afresh.
    public func reset(client: (any HearthClient)? = nil) {
        stop()
        if let client {
            subscribe = Self.subscribe(to: client)
        }
        lastSequence = 0
        task = Task { await self.follow(resyncFirst: true) }
    }

    private func follow(resyncFirst: Bool) async {
        var resyncPending = resyncFirst
        var delay = initialDelay
        while !Task.isCancelled {
            // Open the stream before the handlers refetch, so nothing the core
            // sends while they do is missed.
            let stream = subscribe(lastSequence)
            if resyncPending {
                resyncPending = false
                await deliver(.with { $0.event = .resync(Hearth_V1_Resync()) })
            }
            do {
                for try await event in stream {
                    if Task.isCancelled { return }
                    delay = initialDelay
                    if event.sequence > 0 {
                        lastSequence = event.sequence
                    }
                    await deliver(event)
                }
            } catch {
                // Any failure is a dropped connection; reconnect below.
            }
            if Task.isCancelled { return }
            do {
                try await Task.sleep(for: delay)
            } catch {
                return
            }
            delay = min(delay * 2, maxDelay)
        }
    }

    private func deliver(_ event: Hearth_V1_SubscribeResponse) async {
        for (_, handle) in handlers {
            // A reset while a handler awaited: this event belongs to the old
            // helper, and the new task is already delivering its resync.
            if Task.isCancelled { return }
            await handle(event)
        }
    }

    private static func subscribe(to client: any HearthClient) -> Subscribe {
        { sequence in client.subscribe(after: sequence) }
    }
}
