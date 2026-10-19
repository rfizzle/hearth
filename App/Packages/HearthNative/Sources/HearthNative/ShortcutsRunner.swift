// ShortcutsRunner runs a user's Shortcut through the `shortcuts` command line
// tool, so a bot can reach Mail, Notes and anything else the user has wired
// into Shortcuts without a plugin. It needs the non-sandboxed app: a sandboxed
// process cannot spawn the tool.

import Foundation

/// Where a run is. `waitingForUser` means the shortcut outlived the grace
/// period, which in practice is a prompt nobody can see.
public enum ShortcutsState: Sendable, Equatable {
    case running
    case waitingForUser
    case finished
    case notFound
    case timedOut
    case failed
}

public enum ShortcutsError: Error, Sendable, Equatable {
    /// No shortcut has that name.
    case notFound(String)
    /// The run outlived the grace period and then the timeout: the shortcut is
    /// most likely blocked on a prompt only the person at the screen can answer.
    case waitingForUser
    /// The run hit the timeout before the grace period ended.
    case timedOut
    /// The tool exited non-zero for another reason.
    case failed(exitCode: Int32, message: String)

    /// The state a UI shows for this error.
    public var state: ShortcutsState {
        switch self {
        case .notFound: .notFound
        case .waitingForUser: .waitingForUser
        case .timedOut: .timedOut
        case .failed: .failed
        }
    }
}

public struct ShortcutsRunner: Sendable {
    private let processes: any ProcessRunning
    private let executable: String
    private let grace: Duration

    /// - Parameter grace: how long a run may take before it is reported as
    ///   waiting for the user. The run continues until its timeout.
    public init(
        processes: any ProcessRunning = SystemProcessRunner(),
        executable: String = "/usr/bin/shortcuts",
        grace: Duration = .seconds(10)
    ) {
        self.processes = processes
        self.executable = executable
        self.grace = grace
    }

    /// The names of the user's shortcuts, in the order the tool prints them.
    public func list(timeout: Duration = .seconds(10)) async throws -> [String] {
        let output = try await execute(
            arguments: ["list"], input: "", timeout: timeout, grace: nil, onState: { _ in })
        guard output.exitCode == 0 else { throw failure(output) }
        return output.standardOutput.split(separator: "\n").map(String.init)
    }

    /// Runs a shortcut with `input` and returns the text it outputs.
    /// `onState` hears each transition, starting with `.running`.
    public func run(
        name: String,
        input: String,
        timeout: Duration,
        onState: @Sendable (ShortcutsState) -> Void = { _ in }
    ) async throws -> String {
        onState(.running)
        let output: ProcessOutput
        do {
            output = try await execute(
                arguments: ["run", name, "-i", "-", "-o", "-"], input: input,
                timeout: timeout, grace: grace, onState: onState)
        } catch let error as ShortcutsError {
            onState(error.state)
            throw error
        }
        if output.exitCode == 0 {
            onState(.finished)
            return output.standardOutput
        }
        // The tool prints "Couldn't find shortcut"; match without the apostrophe.
        let error: ShortcutsError =
            output.standardError.contains("find shortcut")
            ? .notFound(name) : failure(output)
        onState(error.state)
        throw error
    }

    private func failure(_ output: ProcessOutput) -> ShortcutsError {
        let message = output.standardError.trimmingCharacters(in: .whitespacesAndNewlines)
        return .failed(exitCode: output.exitCode, message: message)
    }

    private enum Step: Sendable {
        case exited(ProcessOutput)
        case graceElapsed
        case timeoutElapsed
    }

    /// Races the process against the grace period and the timeout. Leaving the
    /// group cancels the process, which the runner turns into a kill.
    private func execute(
        arguments: [String], input: String, timeout: Duration, grace: Duration?,
        onState: @Sendable (ShortcutsState) -> Void
    ) async throws -> ProcessOutput {
        let processes = processes
        let executable = executable
        return try await withThrowingTaskGroup(of: Step.self) { group in
            group.addTask {
                .exited(
                    try await processes.run(
                        executable: executable, arguments: arguments, input: input))
            }
            group.addTask {
                try await Task.sleep(for: timeout)
                return .timeoutElapsed
            }
            if let grace {
                group.addTask {
                    try await Task.sleep(for: grace)
                    return .graceElapsed
                }
            }
            var waiting = false
            defer { group.cancelAll() }
            while let step = try await group.next() {
                switch step {
                case .exited(let output):
                    return output
                case .graceElapsed:
                    waiting = true
                    onState(.waitingForUser)
                case .timeoutElapsed:
                    throw waiting ? ShortcutsError.waitingForUser : .timedOut
                }
            }
            throw ShortcutsError.timedOut
        }
    }
}
