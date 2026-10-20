// What the helper is launched with: its arguments and the stdin handshake that
// carries the launch token and, when there is a native server, its own token.

import Foundation
import Testing

@testable import Hearth

@Test @MainActor func handshakeSendsTheNativeTokenSecond() {
    let both = HelperSupervisor.handshake(token: "launch", nativeToken: "native")
    #expect(String(bytes: both, encoding: .utf8) == "launch\nnative\n")
    let alone = HelperSupervisor.handshake(token: "launch", nativeToken: nil)
    #expect(String(bytes: alone, encoding: .utf8) == "launch\n")
}

@Test @MainActor func nativePortGoesOnArgvNeverAToken() {
    let arguments = HelperSupervisor.arguments(dataDir: "/data", nativePort: 5123)
    #expect(
        arguments == [
            "serve", "--listen", "127.0.0.1:0", "--data-dir", "/data", "--native", "127.0.0.1:5123",
        ])
    #expect(
        HelperSupervisor.arguments(dataDir: nil, nativePort: nil) == [
            "serve", "--listen", "127.0.0.1:0",
        ])
}

@Test @MainActor func theDefaultLaunchCarriesNativeUnlessTurnedOff() throws {
    let name = "hearth-test-\(UUID().uuidString)"
    let defaults = try #require(UserDefaults(suiteName: name))
    defer { defaults.removePersistentDomain(forName: name) }
    #expect(HelperSupervisor.nativeEnabled(defaults))
    defaults.set(false, forKey: "native.recognition")
    #expect(!HelperSupervisor.nativeEnabled(defaults))
    let port: UInt16? = HelperSupervisor.nativeEnabled(defaults) ? 5123 : nil
    #expect(!HelperSupervisor.arguments(dataDir: nil, nativePort: port).contains("--native"))
    defaults.set("NO", forKey: "native.recognition")
    #expect(!HelperSupervisor.nativeEnabled(defaults))
    defaults.set(true, forKey: "native.recognition")
    #expect(HelperSupervisor.nativeEnabled(defaults))
}
