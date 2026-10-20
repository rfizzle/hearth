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
// and passes its port as --native; a port is not a secret. Its bearer is a
// second random token of its own, sent on stdin after the launch token, so a
// process that takes the port after the app lets it go learns nothing that
// controls the helper. If it cannot bind, the helper runs without it and
// says pictures could not be read. If it fails later, the helper is
// restarted with a new one; when the app quits, the listener is dropped only
// after the helper has exited.
//
// The native server is on unless the defaults key native.recognition is set
// to false. It parses nothing itself: text recognition reads files the owner
// did not write, so it runs in the sandboxed reading service bundled in the
// app, which a crafted file can crash without touching the app. Turned off,
// the helper runs without --native and says pictures could not be read.
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
            let native = Self.nativeEnabled() ? await bindNative() : nil
            launching = false
            guard process == nil, !stopping else { return dropNative() }
            launch(token: token, native: native)
        }
    }

    /// Binds the native server with a token of its own and returns its port
    /// and that token, or nil when it could not, so the helper runs without it.
    private func bindNative() async -> (port: UInt16, token: String)? {
        dropNative()
        do {
            let token = try Self.launchToken()
            let server = try NativeServer(token: token, reader: ReadingClient.service()) {
                Task { @MainActor [weak self] in self?.nativeFailed() }
            }
            native = server
            return (try await server.start(), token)
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

    /// The native listener failed after it was ready: the helper still holds
    /// its port, so it is restarted, and the next launch binds a new one.
    private func nativeFailed() {
        log.error("native server failed; restarting the helper")
        process?.terminate()
    }

    /// Whether the native server is bound: the defaults key
    /// native.recognition, true unless set to false.
    static func nativeEnabled(_ defaults: UserDefaults = .standard) -> Bool {
        defaults.object(forKey: "native.recognition") == nil
            || defaults.bool(forKey: "native.recognition")
    }

    /// The helper's arguments: serve, the data folder when one is set, and the
    /// native port when there is a native server.
    static func arguments(dataDir: String?, nativePort: UInt16?) -> [String] {
        var arguments = ["serve", "--listen", "127.0.0.1:0"]
        if let dataDir { arguments += ["--data-dir", dataDir] }
        if let nativePort { arguments += ["--native", "127.0.0.1:\(nativePort)"] }
        return arguments
    }

    /// What the helper reads from stdin before the lifeline: the launch token,
    /// then the native token when there is a native server, a line each.
    static func handshake(token: String, nativeToken: String?) -> Data {
        Data(([token] + (nativeToken.map { [$0] } ?? [])).map { $0 + "\n" }.joined().utf8)
    }

    private func launch(token: String, native: (port: UInt16, token: String)?) {
        let process = Process()
        process.executableURL = Self.helperURL()
        process.arguments = Self.arguments(dataDir: Self.dataDir(), nativePort: native?.port)
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
            try stdin.fileHandleForWriting.write(
                contentsOf: Self.handshake(token: token, nativeToken: native?.token))
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
        // The listener goes once the helper has exited, in helperExited, so
        // its port is never free while the helper may still dial it.
        guard let process else { return dropNative() }
        process.terminate()
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
