// SettingsView is the Settings window: General first, with whether hearth
// shows in the menu bar and the quick ask's global shortcut; Accounts and Web
// Search panes, each the same stack of cards onboarding shows, over the
// session's one AccountsStore, the Models pane over its ModelsStore, and the
// Usage pane over its UsageStore, and the Deliverables pane over its
// TemplatesStore. Accounts shows the configured cards and "Add
// a service", whose list holds the rest. With no helper only General works;
// the second tab says what the helper is doing.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthFeatures
import HearthNative
import HearthUI
import SwiftUI

struct SettingsView: View {
    @Environment(AppModel.self) private var model

    var body: some View {
        TabView {
            pane { GeneralPane() }
                .tabItem { Label("General", systemImage: "gearshape") }
            if let session = model.session {
                let accounts = session.accounts
                Group {
                    pane { SignInPanel(accounts: accounts) }
                        .tabItem { Label("Accounts", systemImage: "person.crop.circle") }
                    pane { ModelsPane(store: session.models) }
                        .tabItem { Label("Models", systemImage: "cpu") }
                    pane { IntegrationsPanel(accounts: accounts) }
                        .tabItem { Label("Web Search", systemImage: "magnifyingglass") }
                    pane { UsagePane(usage: session.usage, bots: session.bots) }
                        .tabItem { Label("Usage", systemImage: "chart.bar") }
                    pane { TemplatesPane(store: session.templates) }
                        .tabItem {
                            Label("Deliverables", systemImage: "chart.bar.doc.horizontal")
                        }
                }
            } else {
                Text(model.supervisor.status)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .padding(Theme.Spacing.xl)
                    .tabItem { Label("Accounts", systemImage: "person.crop.circle") }
            }
        }
        .task(id: model.session.map(ObjectIdentifier.init)) { await model.session?.accounts.load() }
        .frame(width: Theme.Size.sheetWidth)
    }

    private func pane<Content: View>(@ViewBuilder _ content: () -> Content) -> some View {
        ScrollView { content().padding(Theme.Spacing.xl) }
    }
}

/// Whether the menu bar extra shows, and the quick ask's shortcut.
private struct GeneralPane: View {
    @AppStorage(MenuBarExtraScene.shownKey) private var showsInMenuBar = true

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Toggle("Show in menu bar", isOn: $showsInMenuBar)
                Text("Who is working and what needs you, at a glance, with a quick ask.")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                GlobalShortcut.quickAsk.recorder("Quick ask:")
                Text("Opens a one-line ask from any app.")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
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
            onCloseService: { accounts.dismissServiceError() },
            onListModels: { address, key in await accounts.serviceModels(address, key: key) })
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
