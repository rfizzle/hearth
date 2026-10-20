// Store tests for adding a service by address, against PreviewClient and core
// events handed to the store as the relay hands them.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func accountsStore() -> AccountsStore {
    AccountsStore(client: PreviewClient(), relay: EventRelay(client: PreviewClient()))
}

private func changed(_ account: Hearth_V1_ProviderAccount) -> Hearth_V1_SubscribeResponse {
    .with { $0.event = .accountChanged(.with { $0.account = account }) }
}

@MainActor
@Test func aKnownServiceIsAddedAsItsOwnCard() async throws {
    let accounts = accountsStore()
    await accounts.load()
    let builtins = accounts.accounts.count
    let groq = try #require(accounts.knownServices.first { $0.id == "groq" })

    await accounts.saveService(
        .with {
            $0.displayName = groq.name
            $0.baseURL = groq.baseURL
            $0.defaultModel = groq.defaultModel
            $0.key = "gsk-secret-4321"
        })

    #expect(accounts.serviceError == nil)
    #expect(accounts.accounts.count == builtins + 1)
    let added = try #require(accounts.accounts.last)
    #expect(added.provider == .openaiCompatible)
    #expect(added.displayName == "Groq")
    #expect(added.defaultModel == "llama-3.3-70b-versatile")
    #expect(added.label == "4321")
    #expect(accounts.hasSignedInAccount)
}

@MainActor
@Test func aCustomGatewayIsAddedThenRemoved() async throws {
    let accounts = accountsStore()
    let gateway = Hearth_V1_SetAPIKeyRequest.with {
        $0.displayName = "Acme gateway"
        $0.baseURL = "https://llm.acme.example/v1"
        $0.defaultModel = "acme-large"
        $0.key = "gw-secret-9c4e"
    }
    await accounts.saveService(gateway)
    let added = try #require(accounts.accounts.first { $0.provider == .openaiCompatible })
    #expect(added.displayName == "Acme gateway")
    #expect(!added.profile.isEmpty)

    // The core's event for the same service replaces the card, not adds one.
    await accounts.handle(changed(added))
    #expect(accounts.accounts.filter { $0.provider == .openaiCompatible }.count == 1)

    await accounts.removeService(added.profile)
    #expect(!accounts.accounts.contains { $0.provider == .openaiCompatible })
}

@MainActor
@Test func aServiceReportedSignedOutLeavesTheList() async {
    let accounts = accountsStore()
    await accounts.handle(changed(Fixtures.customService))
    #expect(accounts.accounts.contains { $0.profile == Fixtures.customService.profile })

    var removed = Fixtures.customService
    removed.state = .signedOut
    await accounts.handle(changed(removed))
    #expect(accounts.accounts.isEmpty)
}
