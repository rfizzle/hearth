// The fake holds what is saved; the real Keychain is exercised only by the
// contract test, behind HEARTH_CONTRACT, under a service name of its own.

import Foundation
import Testing

@testable import HearthNative

@Suite("Keychain fake")
struct FakeKeychainTests {
    @Test func holdsWhatWasSaved() throws {
        let keychain = FakeKeychain()
        try keychain.save("anthropic", Data("fixture-value-1".utf8))
        try keychain.save("brave-search", Data("fixture-value-2".utf8))
        try keychain.save("anthropic", Data("fixture-value-3".utf8))
        #expect(try keychain.list() == ["anthropic", "brave-search"])
        #expect(try keychain.load("anthropic") == Data("fixture-value-3".utf8))
        try keychain.delete("anthropic")
        try keychain.delete("anthropic")
        #expect(try keychain.load("anthropic") == nil)
    }

    @Test func aFailureNamesTheStatusAndNoValue() {
        let keychain = FakeKeychain()
        keychain.failure = KeychainError(action: "save a key", status: -25308)
        #expect(throws: KeychainError.self) { try keychain.save("a", Data("fixture-value-1".utf8)) }
        let words = keychain.failure?.errorDescription ?? ""
        #expect(words == "Hearth could not save a key in the Keychain (error -25308).")
    }
}

/// Writes, reads, lists and deletes one item in the real Keychain under a
/// service name made for the run, and removes it whatever the assertions say.
@Suite(
    "Keychain contract",
    .enabled(if: ProcessInfo.processInfo.environment["HEARTH_CONTRACT"] == "1"))
struct KeychainContractTests {
    /// The file keychain, since a test process has no keychain access group.
    @Test func theRealKeychainRoundTrips() throws {
        let keychain = Keychain(
            service: "is.hearth.test.\(UUID().uuidString)", dataProtection: false)
        defer { try? keychain.delete("contract-record") }
        try keychain.save("contract-record", Data("fixture-value-1".utf8))
        try keychain.save("contract-record", Data("fixture-value-2".utf8))
        #expect(try keychain.list() == ["contract-record"])
        #expect(try keychain.load("contract-record") == Data("fixture-value-2".utf8))
        try keychain.delete("contract-record")
        #expect(try keychain.load("contract-record") == nil)
        #expect(try keychain.list().isEmpty)
    }

    /// The data protection keychain either works (a signed build with an
    /// access group) or says why it cannot (-34018); it never fails otherwise.
    @Test func theDataProtectionKeychainWorksOrIsRefusedForTheEntitlement() {
        let keychain = Keychain(service: "is.hearth.test.\(UUID().uuidString)")
        defer { try? keychain.delete("contract-record") }
        do {
            try keychain.save("contract-record", Data("fixture-value-1".utf8))
            #expect(throws: Never.self) { try keychain.load("contract-record") }
        } catch let error as KeychainError {
            #expect(error.status == errSecMissingEntitlement)
        } catch {
            Issue.record("unexpected error type")
        }
    }
}
