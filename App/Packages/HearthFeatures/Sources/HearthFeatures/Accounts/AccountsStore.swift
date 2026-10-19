// AccountsStore holds the provider accounts and the integrations as the core
// last reported them, and turns the owner's intents into calls. It caches and
// applies the account and integration changes the core announces; it decides
// nothing. Onboarding uses it now and Settings reuses it later.

import Foundation
import HearthKit
import Observation

/// The accounts and integrations behind the sign-in and integrations panels.
@MainActor
@Observable
public final class AccountsStore {
    /// Every provider and whether it has a credential.
    public private(set) var accounts: [Hearth_V1_ProviderAccount] = []
    /// Every integration and whether it has a key.
    public private(set) var integrations: [Hearth_V1_IntegrationAccount] = []
    /// The ChatGPT sign-in page for the view to open; nil until one starts.
    public private(set) var loginURL: URL?
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// The last failed integration call's text, if one failed.
    public private(set) var integrationError: String?

    private let client: any HearthClient
    private let events: AsyncStream<Hearth_V1_SubscribeResponse>

    /// A store over `client`; `events` is the relay's stream of core events.
    public init(client: any HearthClient, events: AsyncStream<Hearth_V1_SubscribeResponse>) {
        self.client = client
        self.events = events
    }

    /// Whether any provider account is signed in. An integration never counts.
    public var hasSignedInAccount: Bool {
        accounts.contains { $0.state == .signedIn }
    }

    /// What to show under the sign-in panel: a failed call, else an account's
    /// own failure text.
    public var signInFailure: String? {
        error ?? accounts.first { !$0.failure.isEmpty }?.failure
    }

    /// Fetches both lists from the core.
    public func load() async {
        do {
            accounts = try await client.listAccounts()
            integrations = try await client.listIntegrations()
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Applies account and integration changes until the stream ends.
    public func run() async {
        for await event in events {
            apply(event)
        }
    }

    /// Asks the core to start the ChatGPT sign-in and publishes its page.
    public func startChatGPTLogin() async {
        do {
            loginURL = try await client.startChatGPTLogin()
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Stores an API key for `provider`.
    public func setAPIKey(_ key: String, for provider: Hearth_V1_Provider) async {
        do {
            upsert(try await client.setAPIKey(provider: provider, key: key))
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Forgets the credential for `provider`.
    public func clear(_ provider: Hearth_V1_Provider) async {
        do {
            try await client.clearCredential(provider: provider)
            if let index = accounts.firstIndex(where: { $0.provider == provider }) {
                accounts[index].state = .signedOut
                accounts[index].label = ""
                accounts[index].failure = ""
            }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Stores the key for `integration`.
    public func setKey(_ key: String, for integration: Hearth_V1_Integration) async {
        do {
            upsert(try await client.setIntegrationKey(integration, key: key))
            integrationError = nil
        } catch {
            integrationError = error.localizedDescription
        }
    }

    /// Forgets the key for `integration`.
    public func clear(_ integration: Hearth_V1_Integration) async {
        do {
            try await client.clearIntegration(integration)
            if let index = integrations.firstIndex(where: { $0.integration == integration }) {
                integrations[index].configured = false
                integrations[index].label = ""
            }
            integrationError = nil
        } catch {
            integrationError = error.localizedDescription
        }
    }

    private func apply(_ event: Hearth_V1_SubscribeResponse) {
        switch event.event {
        case .accountChanged(let change): upsert(change.account)
        case .integrationChanged(let change): upsert(change.integration)
        default: break
        }
    }

    private func upsert(_ account: Hearth_V1_ProviderAccount) {
        if let index = accounts.firstIndex(where: { $0.provider == account.provider }) {
            accounts[index] = account
        } else {
            accounts.append(account)
        }
    }

    private func upsert(_ account: Hearth_V1_IntegrationAccount) {
        let index = integrations.firstIndex { $0.integration == account.integration }
        if let index {
            integrations[index] = account
        } else {
            integrations.append(account)
        }
    }
}
