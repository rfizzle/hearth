// The handshake and the change events, against a PreviewClient that answers
// from a script and a fake Keychain. Every value is a fixture string.

import Foundation
import HearthKit
import HearthNative
import Testing

@testable import Hearth

private func record(_ name: String, _ value: String) -> Hearth_V1_Credential {
    .with {
        $0.name = name
        $0.value = Data(value.utf8)
    }
}

private func changed(_ name: String, deleted: Bool = false) -> Hearth_V1_SubscribeResponse {
    .with {
        $0.event = .credentialChanged(
            .with {
                $0.name = name
                $0.deleted = deleted
            })
    }
}

@MainActor
private final class Reports {
    var lines: [String] = []
}

@MainActor
private func makeSync(_ keychain: FakeKeychain, _ reports: Reports) -> CredentialSync {
    CredentialSync(keychain: keychain) { reports.lines.append($0) }
}

@Suite("Credential sync")
@MainActor
struct CredentialSyncTests {
    @Test func handshakeSendsWhatTheKeychainHoldsStoresWhatComesBackAndSyncsAgain() async {
        let keychain = FakeKeychain(records: ["anthropic": Data("fixture-value-1".utf8)])
        let script = CredentialScript(toStore: [
            [record("chatgpt", "fixture-value-2"), record("brave-search", "fixture-value-3")]
        ])
        let reports = Reports()
        let sync = makeSync(keychain, reports)

        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()

        #expect(script.syncs.count == 2)
        #expect(script.syncs[0] == [record("anthropic", "fixture-value-1")])
        #expect(
            script.syncs[1].map(\.name) == ["anthropic", "brave-search", "chatgpt"])
        #expect(keychain.records["chatgpt"] == Data("fixture-value-2".utf8))
        #expect(keychain.records["brave-search"] == Data("fixture-value-3".utf8))
        #expect(reports.lines.isEmpty)
    }

    @Test func anEmptyKeychainItemIsLeftOutAndTheRestSync() async {
        let keychain = FakeKeychain(records: [
            "anthropic": Data("fixture-value-1".utf8), "brave-search": Data(),
        ])
        let script = CredentialScript()
        let reports = Reports()
        let sync = makeSync(keychain, reports)
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()
        #expect(script.syncs.count == 2)
        #expect(script.syncs[0].map(\.name) == ["anthropic"])
        #expect(reports.lines.isEmpty)
    }

    @Test func aHandshakeWithNothingToStoreStillSyncsTwice() async {
        let script = CredentialScript()
        let sync = makeSync(FakeKeychain(), Reports())
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()
        #expect(script.syncs == [[], []])
    }

    @Test func aChangeExportsOneRecordAndSavesIt() async {
        let keychain = FakeKeychain()
        let script = CredentialScript(exports: ["chatgpt": record("chatgpt", "fixture-value-4")])
        let sync = makeSync(keychain, Reports())
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()

        await sync.handle(changed("chatgpt"))

        #expect(script.exported == ["chatgpt"])
        #expect(keychain.records == ["chatgpt": Data("fixture-value-4".utf8)])
    }

    @Test func aDeletedChangeDeletesWithoutExporting() async {
        let keychain = FakeKeychain(records: ["anthropic": Data("fixture-value-1".utf8)])
        let script = CredentialScript()
        let sync = makeSync(keychain, Reports())
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()

        await sync.handle(changed("anthropic", deleted: true))

        #expect(script.exported.isEmpty)
        #expect(keychain.records.isEmpty)
    }

    @Test func otherEventsAndEventsWithNoHelperDoNothing() async {
        let keychain = FakeKeychain()
        let script = CredentialScript()
        let sync = makeSync(keychain, Reports())
        await sync.handle(changed("chatgpt"))
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()
        sync.helperGone()
        await sync.handle(changed("chatgpt"))
        await sync.handle(.with { $0.event = .heartbeat(Hearth_V1_Heartbeat()) })
        #expect(script.exported.isEmpty)
        #expect(keychain.records.isEmpty)
    }

    @Test func aKeychainFailureReachesTheStatusLineWithTheStatusAndNoValue() async {
        let keychain = FakeKeychain()
        keychain.failure = KeychainError(action: "read a saved key", status: -25293)
        let reports = Reports()
        let script = CredentialScript(toStore: [[record("chatgpt", "fixture-value-5")]])
        let sync = makeSync(keychain, reports)

        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()

        #expect(
            reports.lines == ["Hearth could not read a saved key in the Keychain (error -25293)."])
        #expect(script.syncs.isEmpty)
        #expect(!reports.lines.joined().contains("fixture-value"))
    }

    @Test func aFailedSaveOfAChangeIsReportedAndKeepsNoValue() async {
        let keychain = FakeKeychain()
        let script = CredentialScript(exports: ["chatgpt": record("chatgpt", "fixture-value-6")])
        let reports = Reports()
        let sync = makeSync(keychain, reports)
        sync.helperStarted(client: PreviewClient(credentials: script))
        await sync.settle()

        keychain.failure = KeychainError(action: "save a key", status: -34018)
        await sync.handle(changed("chatgpt"))

        #expect(reports.lines == ["Hearth could not save a key in the Keychain (error -34018)."])
        keychain.failure = nil
        #expect(keychain.records.isEmpty)
    }
}
