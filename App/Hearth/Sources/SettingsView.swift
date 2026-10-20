// SettingsView is the Settings window: Accounts and Web Search panes, each the
// same stack of cards onboarding shows, over the session's one AccountsStore.
// Accounts shows the configured cards and "Add a service", whose list holds
// the rest. With no helper there is nothing to show yet.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthFeatures
import HearthUI
import SwiftUI

struct SettingsView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        Group {
            if let accounts = model.session?.accounts {
                TabView {
                    pane { SignInPanel(accounts: accounts) }
                        .tabItem { Label("Accounts", systemImage: "person.crop.circle") }
                    pane { IntegrationsPanel(accounts: accounts) }
                        .tabItem { Label("Web Search", systemImage: "magnifyingglass") }
                }
                .task { await accounts.load() }
            } else {
                Text(model.supervisor.status)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .padding(Theme.Spacing.xl)
            }
        }
        .frame(width: Theme.Size.sheetWidth)
    }

    private func pane<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        ScrollView { content().padding(Theme.Spacing.xl) }
    }
}

extension SignInPanel {
    fileprivate init(accounts: AccountsStore) {
        self.init(
            accounts: accounts.accounts, knownServices: accounts.knownServices,
            loginURL: accounts.loginURL, failure: accounts.signInFailure,
            serviceFailure: accounts.serviceError, featured: [],
            onSignInWithChatGPT: { Task { await accounts.startChatGPTLogin() } },
            onSetKey: { provider, key in Task { await accounts.setAPIKey(key, for: provider) } },
            onClear: { provider in Task { await accounts.clear(provider) } },
            onSaveService: { request in Task { await accounts.saveService(request) } },
            onRemoveService: { profile in Task { await accounts.removeService(profile) } },
            onCloseService: { accounts.dismissServiceError() })
    }
}

extension IntegrationsPanel {
    fileprivate init(accounts: AccountsStore) {
        self.init(
            integrations: accounts.integrations, failure: accounts.integrationError,
            onSetKey: { integration, key in Task { await accounts.setKey(key, for: integration) } },
            onClear: { integration in Task { await accounts.clear(integration) } })
    }
}
