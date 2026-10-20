// HelperSupervisor owns hearthd's lifecycle: it launches the bundled helper
// with a fresh token, reads the loopback address the helper prints, publishes
// a LiveClient for it, and relaunches the helper one second after it exits
// unless the app is quitting.
//
// The helper's stdin carries the token and is the lifeline: the app writes the
// token and a newline right after launch, then holds the write end open and
// writes nothing more, so when the app quits or crashes the kernel closes it
// and the helper exits. HTTP connections are per request and cannot play that
// part. The token goes over the pipe rather than argv because any local
// process can read another's argv with `ps`. It is new on every launch
// because it is the only thing that keeps another local process off the
// helper's port.
//
// Before each launch the supervisor binds the native server, the app's own
// loopback listener the core calls back into (to read pictures in documents),
// with the same token as its bearer, and passes its port as --native. A port
// is not a secret. If it cannot bind, the helper runs without it and says
// pictures could not be read.
// See docs/architecture.md#5-security-model

import Foundation
import HearthKit
import HearthNative
import Observation
import Security
import os

@MainActor
@Observable
final class HelperSupervisor {
    /// The client for the running helper; nil while it starts or restarts.
    private(set) var client: LiveClient?
    /// Bumped each time a new helper is ready, so views can stream again.
    private(set) var generation = 0
    /// What the window shows while there is no client.
    private(set) var status = "Starting…"

    @ObservationIgnored private var process: Process?
    @ObservationIgnored private var native: NativeServer?
    @ObservationIgnored private var launching = false
    @ObservationIgnored private var lifeline: FileHandle?
    @ObservationIgnored private var stopping = false
    @ObservationIgnored private let log = Logger(
        subsystem: "com.rfizzle.hearth", category: "helper")

    /// Launches the helper unless one is already running. Calling it again is
    /// safe and does nothing: the app calls it at launch and the main window
    /// each time it is made, which the menu bar extra can do many times.
    func start() {
        guard process == nil, !launching, !stopping else { return }
        let token: String
        do {
            token = try Self.launchToken()
        } catch {
            status = "Could not create a launch token: \(error.localizedDescription)"
            return
        }
        launching = true
        Task {
            let port = await bindNative(token: token)
            launching = false
            guard process == nil, !stopping else { return dropNative() }
            launch(token: token, nativePort: port)
        }
    }

    /// Binds the native server for this launch's token and returns its port,
    /// or nil when it could not bind, so the helper runs without it.
    private func bindNative(token: String) async -> UInt16? {
        dropNative()
        do {
            let server = try NativeServer(token: token, extractor: TextExtractor())
            native = server
            return try await server.start()
        } catch {
            log.error(
                "native server did not start: \(error.localizedDescription, privacy: .public)")
            dropNative()
            return nil
        }
    }

    private func dropNative() {
        native?.stop()
        native = nil
    }

    private func launch(token: String, nativePort: UInt16?) {
        let process = Process()
        process.executableURL = Self.helperURL()
        process.arguments = ["serve", "--listen", "127.0.0.1:0"]
        if let dataDir = Self.dataDir() {
            process.arguments?.append(contentsOf: ["--data-dir", dataDir])
        }
        if let nativePort {
            process.arguments?.append(contentsOf: ["--native", "127.0.0.1:\(nativePort)"])
        }
        let stdin = Pipe()
        let stdout = Pipe()
        process.standardInput = stdin
        process.standardOutput = stdout
        process.standardError = FileHandle.standardError
        process.terminationHandler = { [weak self] exited in
            let code = exited.terminationStatus
            Task { @MainActor in self?.helperExited(status: code) }
        }
        do {
            try process.run()
        } catch {
            status = "Could not launch the helper: \(error.localizedDescription)"
            dropNative()
            return
        }
        self.process = process
        self.lifeline = stdin.fileHandleForWriting
        do {
            try stdin.fileHandleForWriting.write(contentsOf: Data((token + "\n").utf8))
        } catch {
            status = "Could not send the launch token: \(error.localizedDescription)"
            process.terminate()
            return
        }
        log.notice("helper launched, pid \(process.processIdentifier)")
        Task { await self.connect(reading: stdout.fileHandleForReading, token: token) }
    }

    /// Puts a sentence on the status line, for a failure that is not the
    /// helper's own (the credential sync). It names no key and no value.
    func report(_ message: String) {
        status = message
        log.error("\(message, privacy: .public)")
    }

    /// Stops the helper for good: closes the lifeline and terminates it.
    func stop() {
        stopping = true
        try? lifeline?.close()
        lifeline = nil
        process?.terminate()
        dropNative()
    }

    private func connect(reading stdout: FileHandle, token: String) async {
        var address: String?
        do {
            for try await line in stdout.bytes.lines {
                address = line
                break
            }
        } catch {
            status = "Could not read the helper's address: \(error.localizedDescription)"
            return
        }
        guard let address, let url = URL(string: "http://\(address)") else {
            status = "The helper did not report an address."
            return
        }
        log.notice("helper listening at \(address, privacy: .public)")
        client = LiveClient(baseURL: url, token: token)
        generation += 1
    }

    private func helperExited(status code: Int32) {
        process = nil
        try? lifeline?.close()
        lifeline = nil
        client = nil
        dropNative()
        log.notice("helper exited with status \(code)")
        guard !stopping else { return }
        status = "The helper stopped; restarting…"
        Task {
            try? await Task.sleep(for: .seconds(1))
            start()
        }
    }

    /// The helper inside the app bundle, or HEARTH_HELPER when set (for
    /// running the app outside a bundle).
    private static func helperURL() -> URL {
        if let path = ProcessInfo.processInfo.environment["HEARTH_HELPER"] {
            return URL(filePath: path)
        }
        return Bundle.main.bundleURL.appending(path: "Contents/Helpers/hearthd")
    }

    /// HEARTH_DATA_DIR when set: where the helper keeps its database and
    /// credentials instead of the Hearth folder in Application Support, so a
    /// trial run leaves the owner's own data alone.
    private static func dataDir() -> String? {
        let path = ProcessInfo.processInfo.environment["HEARTH_DATA_DIR"]
        return path?.isEmpty == false ? path : nil
    }

    /// 32 random bytes as hex.
    private static func launchToken() throws -> String {
        var bytes = [UInt8](repeating: 0, count: 32)
        let result = SecRandomCopyBytes(kSecRandomDefault, bytes.count, &bytes)
        guard result == errSecSuccess else {
            throw NSError(domain: NSOSStatusErrorDomain, code: Int(result))
        }
        return bytes.map { String(format: "%02x", $0) }.joined()
    }
}
