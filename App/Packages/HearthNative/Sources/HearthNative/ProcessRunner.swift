// ProcessRunner is the seam between a native tool and the operating system:
// a tool describes a command, a runner executes it. The real runner spawns a
// process; tests substitute a stub, so no test runs a system binary.

import Foundation

/// What a finished process left behind.
public struct ProcessOutput: Sendable, Equatable {
    public var exitCode: Int32
    public var standardOutput: String
    public var standardError: String

    public init(exitCode: Int32, standardOutput: String = "", standardError: String = "") {
        self.exitCode = exitCode
        self.standardOutput = standardOutput
        self.standardError = standardError
    }
}

/// Runs one command to completion. An implementation must stop the process
/// when the calling task is cancelled; callers rely on that to enforce
/// timeouts.
public protocol ProcessRunning: Sendable {
    func run(executable: String, arguments: [String], input: String) async throws -> ProcessOutput
}

/// Spawns a real process. The input is always written and the pipe closed,
/// even when empty, because a child that reads standard input otherwise
/// waits for it forever.
public struct SystemProcessRunner: ProcessRunning {
    public init() {}

    public func run(
        executable: String, arguments: [String], input: String
    ) async throws -> ProcessOutput {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: executable)
        process.arguments = arguments
        let stdin = Pipe()
        let stdout = Pipe()
        let stderr = Pipe()
        process.standardInput = stdin
        process.standardOutput = stdout
        process.standardError = stderr

        let (exits, exited) = AsyncStream.makeStream(of: Int32.self)
        process.terminationHandler = { exited.yield($0.terminationStatus) }
        try process.run()

        // The child holds its own copies; ours must close or the reads never
        // see end of file.
        try? stdout.fileHandleForWriting.close()
        try? stderr.fileHandleForWriting.close()
        try? stdin.fileHandleForReading.close()

        // A write to a child that already exited must fail, not raise SIGPIPE.
        _ = fcntl(stdin.fileHandleForWriting.fileDescriptor, F_SETNOSIGPIPE, 1)
        let writer = stdin.fileHandleForWriting
        let data = Data(input.utf8)
        Task.detached {
            try? writer.write(contentsOf: data)
            try? writer.close()
        }
        let outReader = stdout.fileHandleForReading
        let errReader = stderr.fileHandleForReading
        let out = Task.detached { (try? outReader.readToEnd()) ?? Data() }
        let err = Task.detached { (try? errReader.readToEnd()) ?? Data() }

        let handle = Terminator(process)
        var status: Int32 = -1
        await withTaskCancellationHandler {
            for await code in exits {
                status = code
                break
            }
        } onCancel: {
            handle.terminate()
        }
        let outData = await out.value
        let errData = await err.value
        let result = ProcessOutput(
            exitCode: status,
            standardOutput: String(bytes: outData, encoding: .utf8) ?? "",
            standardError: String(bytes: errData, encoding: .utf8) ?? "")
        try Task.checkCancellation()
        return result
    }
}

/// Lets the cancellation handler, which is `@Sendable`, stop the process.
private final class Terminator: @unchecked Sendable {
    private let process: Process
    init(_ process: Process) { self.process = process }
    func terminate() {
        if process.isRunning { process.terminate() }
    }
}
