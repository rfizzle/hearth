// Store tests for onboarding and accounts, against PreviewClient and core
// events handed to the store as the relay hands them.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func stores() -> (accounts: AccountsStore, onboarding: OnboardingStore) {
    let relay = EventRelay(client: PreviewClient())
    let accounts = AccountsStore(client: PreviewClient(), relay: relay)
    let onboarding = OnboardingStore(
        client: PreviewClient(), accounts: accounts, home: URL(filePath: "/Users/test"))
    return (accounts, onboarding)
}

/// Hands `events` to `accounts` one at a time, as the relay does.
@MainActor
private func replay(_ events: [Hearth_V1_SubscribeResponse], into accounts: AccountsStore) async {
    for event in events {
        await accounts.handle(event)
    }
}

private func accountEvent(
    _ state: Hearth_V1_AccountState, failure: String = ""
) -> Hearth_V1_SubscribeResponse {
    .with {
        $0.event = .accountChanged(
            .with {
                $0.account = .with {
                    $0.provider = .chatgpt
                    $0.state = state
                    $0.failure = failure
                }
            })
    }
}

private func integrationEvent() -> Hearth_V1_SubscribeResponse {
    .with {
        $0.event = .integrationChanged(
            .with {
                $0.integration = .with {
                    $0.integration = .braveSearch
                    $0.configured = true
                    $0.label = "1234"
                }
            })
    }
}

@MainActor
@Test func continueWaitsForASignedInAccount() async {
    let (accounts, onboarding) = stores()
    onboarding.next()
    #expect(onboarding.step == .signIn)
    #expect(!onboarding.canContinue)
    await replay([accountEvent(.signedOut), accountEvent(.signedIn)], into: accounts)
    #expect(onboarding.canContinue)
    onboarding.next()
    #expect(onboarding.step == .nameAndFolder)
}

@MainActor
@Test func anIntegrationAloneDoesNotEnableContinue() async {
    let (accounts, onboarding) = stores()
    onboarding.next()
    await replay([integrationEvent()], into: accounts)
    #expect(accounts.integrations.first?.configured == true)
    #expect(!onboarding.canContinue)
    onboarding.next()
    #expect(onboarding.step == .signIn)
}

@MainActor
@Test func aSignInFailureShowsItsText() async {
    let (accounts, _) = stores()
    await replay([accountEvent(.signedOut, failure: "The page was closed.")], into: accounts)
    #expect(accounts.signInFailure == "The page was closed.")
}

@MainActor
@Test func startingChatGPTPublishesThePage() async {
    let (accounts, _) = stores()
    await accounts.startChatGPTLogin()
    #expect(accounts.loginURL == Fixtures.authorizeURL)
}

@MainActor
@Test func settingTheSearchKeyShowsTheRowConfigured() async {
    let (accounts, _) = stores()
    await accounts.load()
    #expect(accounts.integrations.first?.configured == false)
    await accounts.setKey("BSA-1234", for: .braveSearch)
    #expect(accounts.integrations.first?.configured == true)
    #expect(accounts.integrations.first?.label == "1234")
    await accounts.clear(.braveSearch)
    #expect(accounts.integrations.first?.configured == false)
}

@MainActor
@Test func settingAnAPIKeySignsTheProviderIn() async {
    let (accounts, onboarding) = stores()
    await accounts.setAPIKey("sk-abcd", for: .anthropic)
    #expect(accounts.hasSignedInAccount)
    onboarding.next()
    #expect(onboarding.canContinue)
}

@MainActor
@Test func doneSendsTheNameAndFolder() async {
    let (accounts, onboarding) = stores()
    #expect(onboarding.path == "/Users/test/Documents/Hearth")
    await replay([accountEvent(.signedIn)], into: accounts)
    onboarding.next()
    onboarding.next()
    #expect(onboarding.step == .nameAndFolder)
    #expect(!onboarding.canFinish)
    onboarding.name = "Harbor Street Studio"
    onboarding.path = "/Users/test/Elsewhere"
    await onboarding.done()
    #expect(onboarding.org?.name == "Harbor Street Studio")
    #expect(onboarding.org?.workspacePath == "/Users/test/Elsewhere")
    #expect(onboarding.error == nil)
}

@MainActor
@Test func aResyncFromTheRelayRefetchesTheAccounts() async throws {
    let relay = EventRelay(subscribe: { _ in
        AsyncThrowingStream { continuation in
            continuation.yield(
                .with {
                    $0.sequence = 1
                    $0.event = .resync(Hearth_V1_Resync())
                })
        }
    })
    let accounts = AccountsStore(client: PreviewClient(), relay: relay)
    #expect(accounts.accounts.isEmpty)
    relay.start()
    for _ in 0..<400 where accounts.accounts.isEmpty || accounts.integrations.isEmpty {
        try await Task.sleep(for: .milliseconds(5))
    }
    relay.stop()
    #expect(accounts.accounts == Fixtures.accounts)
    #expect(accounts.integrations == Fixtures.integrations)
}
