// The runner is tested against a stub process runner: no test spawns the real
// `shortcuts` binary.

import Foundation
import Testing

@testable import HearthNative

/// Records each call and answers with a canned result, optionally after a
/// delay that cancellation interrupts.
private final class StubRunner: ProcessRunning, @unchecked Sendable {
    struct Call: Equatable {
        var executable: String
        var arguments: [String]
        var input: String
    }

    private let lock = NSLock()
    private var recorded: [Call] = []
    private var cancelled = false
    private let delay: Duration?
    private let result: ProcessOutput

    init(_ result: ProcessOutput, delay: Duration? = nil) {
        self.result = result
        self.delay = delay
    }

    var calls: [Call] { lock.withLock { recorded } }
    var wasCancelled: Bool { lock.withLock { cancelled } }

    func run(executable: String, arguments: [String], input: String) async throws -> ProcessOutput {
        lock.withLock {
            recorded.append(Call(executable: executable, arguments: arguments, input: input))
        }
        if let delay {
            do {
                try await Task.sleep(for: delay)
            } catch {
                lock.withLock { cancelled = true }
                throw error
            }
        }
        return result
    }
}

/// Collects the states a run reports.
private final class States: @unchecked Sendable {
    private let lock = NSLock()
    private var seen: [ShortcutsState] = []
    func add(_ state: ShortcutsState) { lock.withLock { seen.append(state) } }
    var all: [ShortcutsState] { lock.withLock { seen } }
}

@Test func listSplitsLines() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 0, standardOutput: "Alpha\nBeta Two\n"))
    let names = try await ShortcutsRunner(processes: stub).list()
    #expect(names == ["Alpha", "Beta Two"])
    #expect(stub.calls.map(\.arguments) == [["list"]])
}

@Test func listOfNothingIsEmpty() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 0))
    #expect(try await ShortcutsRunner(processes: stub).list().isEmpty)
}

@Test func runReturnsOutputAndPassesInput() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 0, standardOutput: "done"))
    let states = States()
    let out = try await ShortcutsRunner(processes: stub, executable: "/x/shortcuts")
        .run(name: "Echo", input: "hello", timeout: .seconds(5), onState: states.add)
    #expect(out == "done")
    #expect(
        stub.calls == [
            .init(
                executable: "/x/shortcuts", arguments: ["run", "Echo", "-i", "-", "-o", "-"],
                input: "hello")
        ])
    #expect(states.all == [.running, .finished])
}

@Test func missingShortcutIsNotFound() async throws {
    let stub = StubRunner(
        ProcessOutput(
            exitCode: 1,
            standardError: "Error: The operation couldn’t be completed. Couldn’t find shortcut\n"))
    let states = States()
    await #expect(throws: ShortcutsError.notFound("Nope")) {
        try await ShortcutsRunner(processes: stub)
            .run(name: "Nope", input: "", timeout: .seconds(5), onState: states.add)
    }
    #expect(states.all == [.running, .notFound])
}

@Test func otherFailureCarriesExitCodeAndMessage() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 2, standardError: "boom\n"))
    await #expect(throws: ShortcutsError.failed(exitCode: 2, message: "boom")) {
        try await ShortcutsRunner(processes: stub).run(name: "A", input: "", timeout: .seconds(5))
    }
}

@Test func slowRunReportsWaitingThenGivesUp() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 0), delay: .seconds(30))
    let states = States()
    let runner = ShortcutsRunner(processes: stub, grace: .milliseconds(30))
    await #expect(throws: ShortcutsError.waitingForUser) {
        try await runner.run(
            name: "Asks", input: "", timeout: .milliseconds(200), onState: states.add)
    }
    #expect(states.all == [.running, .waitingForUser, .waitingForUser])
    #expect(stub.wasCancelled)
}

@Test func runInsideGraceNeverReportsWaiting() async throws {
    let stub = StubRunner(
        ProcessOutput(exitCode: 0, standardOutput: "ok"), delay: .milliseconds(10))
    let states = States()
    let out = try await ShortcutsRunner(processes: stub, grace: .seconds(5))
        .run(name: "Quick", input: "", timeout: .seconds(10), onState: states.add)
    #expect(out == "ok")
    #expect(states.all == [.running, .finished])
}

@Test func timeoutBeforeGraceIsTimedOut() async throws {
    let stub = StubRunner(ProcessOutput(exitCode: 0), delay: .seconds(30))
    let runner = ShortcutsRunner(processes: stub, grace: .seconds(60))
    await #expect(throws: ShortcutsError.timedOut) {
        try await runner.run(name: "Slow", input: "", timeout: .milliseconds(50))
    }
    #expect(stub.wasCancelled)
}
