// EventRelay against a scripted stream: the first connection fails after two
// events, the second opens with a resync, and reset starts again from 0.

import Foundation
import Synchronization
import Testing

@testable import HearthKit

/// A subscribe that answers each call with the next script, keeping every
/// stream open unless its script ends it.
private final class ScriptedSubscribe: Sendable {
    typealias Stream = AsyncThrowingStream<Hearth_V1_SubscribeResponse, Error>
    typealias Continuation = Stream.Continuation

    let scripts: [@Sendable (Continuation) -> Void]
    private let state = Mutex<(calls: [UInt64], open: [Continuation])>(([], []))

    init(_ scripts: [@Sendable (Continuation) -> Void]) {
        self.scripts = scripts
    }

    /// The sequence each subscribe asked to resume after, in order.
    var calls: [UInt64] {
        state.withLock { $0.calls }
    }

    func callAsFunction(_ after: UInt64) -> Stream {
        Stream { continuation in
            let index = state.withLock { state in
                state.calls.append(after)
                state.open.append(continuation)
                return state.calls.count - 1
            }
            if index < scripts.count {
                scripts[index](continuation)
            }
        }
    }
}

/// What a handler saw.
@MainActor
private final class Recorder {
    var sequences: [UInt64] = []
    var resyncs = 0

    func record(_ event: Hearth_V1_SubscribeResponse) {
        sequences.append(event.sequence)
        if case .resync = event.event { resyncs += 1 }
    }
}

private func event(
    _ sequence: UInt64, _ body: Hearth_V1_SubscribeResponse.OneOf_Event
) -> Hearth_V1_SubscribeResponse {
    .with {
        $0.sequence = sequence
        $0.event = body
    }
}

/// Waits up to two seconds for `condition`.
@MainActor
private func eventually(_ condition: () -> Bool) async throws {
    for _ in 0..<400 where !condition() {
        try await Task.sleep(for: .milliseconds(5))
    }
}

@MainActor
@Test func aFailedStreamResumesFromTheLastSequenceAndResyncReachesEveryHandler() async throws {
    let subscribe = ScriptedSubscribe([
        { stream in
            stream.yield(event(1, .heartbeat(Hearth_V1_Heartbeat())))
            stream.yield(event(2, .runChanged(.with { $0.run = Fixtures.workingRun })))
            stream.finish(throwing: URLError(.networkConnectionLost))
        },
        { stream in
            stream.yield(event(3, .resync(Hearth_V1_Resync())))
            stream.yield(event(4, .heartbeat(Hearth_V1_Heartbeat())))
        },
    ])
    let relay = EventRelay(
        subscribe: { subscribe($0) }, initialDelay: .milliseconds(1), maxDelay: .milliseconds(1))
    let first = Recorder()
    let second = Recorder()
    relay.register { first.record($0) }
    relay.register { second.record($0) }

    relay.start()
    try await eventually { second.sequences.count == 4 }
    relay.stop()

    #expect(subscribe.calls == [0, 2])
    #expect(relay.lastSequence == 4)
    #expect(first.sequences == [1, 2, 3, 4])
    #expect(second.sequences == [1, 2, 3, 4])
    #expect(first.resyncs == 1)
    #expect(second.resyncs == 1)
}

@MainActor
@Test func resetStartsFromZeroAndTellsHandlersToRefetch() async throws {
    let subscribe = ScriptedSubscribe([
        { stream in stream.yield(event(7, .heartbeat(Hearth_V1_Heartbeat()))) },
        { stream in stream.yield(event(1, .heartbeat(Hearth_V1_Heartbeat()))) },
    ])
    let relay = EventRelay(subscribe: { subscribe($0) })
    let recorder = Recorder()
    relay.register { recorder.record($0) }

    relay.start()
    try await eventually { recorder.sequences == [7] }
    relay.reset()
    try await eventually { recorder.sequences.count == 3 }
    relay.stop()

    #expect(subscribe.calls == [0, 0])
    #expect(recorder.sequences == [7, 0, 1])
    #expect(recorder.resyncs == 1)
    #expect(relay.lastSequence == 1)
}

@MainActor
@Test func anUnregisteredHandlerSeesNothing() async throws {
    let subscribe = ScriptedSubscribe([
        { stream in stream.yield(event(1, .heartbeat(Hearth_V1_Heartbeat()))) }
    ])
    let relay = EventRelay(subscribe: { subscribe($0) })
    let kept = Recorder()
    let dropped = Recorder()
    relay.register { kept.record($0) }
    let registration = relay.register { dropped.record($0) }
    relay.unregister(registration)

    relay.start()
    try await eventually { kept.sequences == [1] }
    relay.stop()

    #expect(kept.sequences == [1])
    #expect(dropped.sequences.isEmpty)
}
