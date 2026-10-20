// AccountsStore holds the provider accounts, the services the owner added by
// address, the known services and the integrations as the core last reported
// them, and turns the owner's intents into calls. It caches and applies the
// account and integration changes the relay delivers, and refetches the lists
// on a resync; it decides nothing. An added service is an account of its own,
// told apart by its profile id, and leaves the list when the core reports it
// signed out. Onboarding and Settings share it.

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
    /// The services the core knows the address of, for the add-a-service list.
    public private(set) var knownServices: [Hearth_V1_KnownService] = []
    /// The last failed add-a-service call's text, if one failed.
    public private(set) var serviceError: String?

    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay) {
        self.client = client
        self.relay = relay
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
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
            knownServices = try await client.listKnownServices()
            error = nil
        } catch {
            self.error = error.localizedDescription
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

    /// Adds an OpenAI-compatible service, or changes the one
    /// `request.profile` names; the core checks it and words any refusal.
    public func saveService(_ request: Hearth_V1_SetAPIKeyRequest) async {
        do {
            upsert(try await client.saveService(request))
            serviceError = nil
        } catch {
            serviceError = error.localizedDescription
        }
    }

    /// Removes the added service `profile`.
    public func removeService(_ profile: String) async {
        do {
            try await client.removeService(profile: profile)
            accounts.removeAll { $0.provider == .openaiCompatible && $0.profile == profile }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// The model ids the service at `address` serves for `key`, so the card
    /// can offer them before Save; nil when the list could not be read, and
    /// the card then takes a typed id. With no key typed, an address that is
    /// one saved service's asks that service by its id, so the core uses the
    /// key it holds and the app never has it.
    public func serviceModels(_ address: String, key: String) async -> [String]? {
        let saved = key.isEmpty ? savedService(at: address) : nil
        return try? await client.listServiceModels(
            .with {
                if let saved {
                    $0.profile = saved.profile
                } else {
                    $0.baseURL = address
                    $0.key = key
                }
            })
    }

    /// The one saved service whose address is `address`, ignoring a trailing
    /// slash; none when no service or several have it.
    private func savedService(at address: String) -> Hearth_V1_ProviderAccount? {
        func plain(_ text: String) -> String {
            var text = text.trimmingCharacters(in: .whitespaces)
            while text.hasSuffix("/") { text.removeLast() }
            return text
        }
        let found = accounts.filter {
            $0.provider == .openaiCompatible && !$0.profile.isEmpty
                && plain($0.baseURL) == plain(address)
        }
        return found.count == 1 ? found[0] : nil
    }

    /// Forgets the last add-a-service failure, when the owner closes the card.
    public func dismissServiceError() {
        serviceError = nil
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

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .accountChanged(let change): upsert(change.account)
        case .integrationChanged(let change): upsert(change.integration)
        case .resync: await load()
        default: break
        }
    }

    /// An account is one provider's, or one added service's by its profile.
    /// An added service the core reports signed out has been removed.
    private func upsert(_ account: Hearth_V1_ProviderAccount) {
        let index = accounts.firstIndex {
            $0.provider == account.provider && $0.profile == account.profile
        }
        let removed = account.provider == .openaiCompatible && account.state == .signedOut
        switch (index, removed) {
        case (let index?, true): accounts.remove(at: index)
        case (let index?, false): accounts[index] = account
        case (nil, false): accounts.append(account)
        case (nil, true): break
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
