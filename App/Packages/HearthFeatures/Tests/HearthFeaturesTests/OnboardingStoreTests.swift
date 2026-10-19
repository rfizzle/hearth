// Store tests for onboarding and accounts, against PreviewClient and a
// scripted stream of core events.

import Foundation
import HearthKit
import Testing

@testable import HearthFeatures

@MainActor
private func stores(
    events: [Hearth_V1_SubscribeResponse] = []
) -> (accounts: AccountsStore, onboarding: OnboardingStore) {
    let stream = AsyncStream<Hearth_V1_SubscribeResponse> { continuation in
        for event in events { continuation.yield(event) }
        continuation.finish()
    }
    let accounts = AccountsStore(client: PreviewClient(), events: stream)
    let onboarding = OnboardingStore(
        client: PreviewClient(), accounts: accounts, home: URL(filePath: "/Users/test"))
    return (accounts, onboarding)
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
    let (accounts, onboarding) = stores(events: [accountEvent(.signedOut), accountEvent(.signedIn)])
    onboarding.next()
    #expect(onboarding.step == .signIn)
    #expect(!onboarding.canContinue)
    await accounts.run()
    #expect(onboarding.canContinue)
    onboarding.next()
    #expect(onboarding.step == .nameAndFolder)
}

@MainActor
@Test func anIntegrationAloneDoesNotEnableContinue() async {
    let (accounts, onboarding) = stores(events: [integrationEvent()])
    onboarding.next()
    await accounts.run()
    #expect(accounts.integrations.first?.configured == true)
    #expect(!onboarding.canContinue)
    onboarding.next()
    #expect(onboarding.step == .signIn)
}

@MainActor
@Test func aSignInFailureShowsItsText() async {
    let (accounts, _) = stores(events: [accountEvent(.signedOut, failure: "The page was closed.")])
    await accounts.run()
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
    let (accounts, onboarding) = stores(events: [accountEvent(.signedIn)])
    #expect(onboarding.path == "/Users/test/Documents/Hearth")
    await accounts.run()
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
