// OnboardingScreen shows the store's step: a welcome, the sign-in panel with
// the optional web-search panel under it, and the name and folder form. The
// folder is picked with the system file importer limited to directories, so
// the owner sees the standard sheet and keyboard access is the system's. The
// screen is a function of store state; its only local state is whether the
// importer is open.

import HearthKit
import HearthUI
import SwiftUI
import UniformTypeIdentifiers

/// The first-run flow that names the org and picks its folder.
public struct OnboardingScreen: View {
    let store: OnboardingStore

    @State private var isChoosingFolder = false

    public init(store: OnboardingStore) {
        self.store = store
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            switch store.step {
            case .welcome: welcome
            case .signIn: signIn
            case .nameAndFolder: nameAndFolder
            }
            if let error = store.error {
                Text(error).foregroundStyle(.primary)
            }
            buttons
        }
        .padding()
        .frame(maxWidth: 520)
        .task { await store.accounts.load() }
        .task { await store.accounts.run() }
    }

    private var welcome: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Welcome to Hearth").font(.largeTitle)
            Text("Run a small office of AI employees. Three quick steps and it is yours.")
        }
    }

    private var signIn: some View {
        let accounts = store.accounts
        return VStack(alignment: .leading, spacing: 16) {
            Text("Sign in").font(.title)
            SignInPanel(
                accounts: accounts.accounts, loginURL: accounts.loginURL,
                failure: accounts.signInFailure,
                onSignInWithChatGPT: { Task { await accounts.startChatGPTLogin() } },
                onSetKey: { provider, key in Task { await accounts.setAPIKey(key, for: provider) }
                },
                onClear: { provider in Task { await accounts.clear(provider) } })
            Text("Optional: web search").font(.headline)
            IntegrationsPanel(
                integrations: accounts.integrations, failure: accounts.integrationError,
                onSetKey: { integration, key in
                    Task { await accounts.setKey(key, for: integration) }
                },
                onClear: { integration in Task { await accounts.clear(integration) } })
        }
    }

    private var nameAndFolder: some View {
        @Bindable var store = store
        return VStack(alignment: .leading, spacing: 12) {
            Text("Name your org").font(.title)
            TextField("Org name", text: $store.name)
            HStack {
                Text(store.path).lineLimit(1).truncationMode(.middle)
                Button("Choose…") { isChoosingFolder = true }
            }
        }
        .fileImporter(isPresented: $isChoosingFolder, allowedContentTypes: [.folder]) { result in
            if case .success(let url) = result { store.path = url.path }
        }
    }

    private var buttons: some View {
        HStack {
            if store.step != .welcome {
                Button("Back") { store.back() }
            }
            Spacer()
            if store.step == .nameAndFolder {
                Button("Done") { Task { await store.done() } }
                    .buttonStyle(.borderedProminent)
                    .disabled(!store.canFinish)
            } else {
                Button("Continue") { store.next() }
                    .buttonStyle(.borderedProminent)
                    .disabled(!store.canContinue)
            }
        }
    }
}
