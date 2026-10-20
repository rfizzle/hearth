// OnboardingScreen shows the store's step in a centred column: a welcome, the
// sign-in cards with the optional web-search card folded away under them, and
// the name and folder form. Optional setup waits behind a collapsed
// disclosure so the owner is not asked for it before they can continue. The
// folder is picked with the system file importer limited to directories, so
// the owner sees the standard sheet and keyboard access is the system's. The
// screen is a function of store state; its only local state is whether the
// importer is open and whether the web-search disclosure is.
// See docs/design.md#empty-states-and-first-run

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI
import UniformTypeIdentifiers

/// The first-run flow that names the org and picks its folder.
public struct OnboardingScreen: View {
    let store: OnboardingStore

    @State private var isChoosingFolder = false
    @State private var isWebSearchOpen = false

    public init(store: OnboardingStore) {
        self.store = store
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            header
            switch store.step {
            case .welcome: EmptyView()
            case .signIn: signIn
            case .nameAndFolder: nameAndFolder
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
            buttons
        }
        .frame(maxWidth: Theme.Size.formWidth)
        .padding(.top, Theme.Spacing.xl)
        .padding(.horizontal, Theme.Spacing.lg)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .top)
        .navigationTitle("Welcome")
        .task { await store.accounts.load() }
    }

    private var header: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            if store.step == .welcome {
                Image(systemName: "flame.fill")
                    .font(Theme.largeTitle)
                    .foregroundStyle(.tint)
                    .padding(.bottom, Theme.Spacing.sm)
                    .accessibilityHidden(true)
            }
            Text("Step \(store.step.rawValue + 1) of \(Self.stepCount)")
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            Text(heading)
                .font(Theme.title)
                .foregroundStyle(Theme.color(.textPrimary))
                .accessibilityAddTraits(.isHeader)
            Text(lead)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
        }
    }

    private static let stepCount = OnboardingStore.Step.allCases.count

    private var heading: String {
        switch store.step {
        case .welcome: "Run a small office of AI employees."
        case .signIn: "Sign in"
        case .nameAndFolder: "Name your office"
        }
    }

    private var lead: String {
        switch store.step {
        case .welcome: "Three quick steps and it is yours."
        case .signIn: "Choose a service for your team to think with."
        case .nameAndFolder: "Give it a name and a folder to keep its files in."
        }
    }

    private var signIn: some View {
        let accounts = store.accounts
        return VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            SignInPanel(
                accounts: accounts.accounts, loginURL: accounts.loginURL,
                failure: accounts.signInFailure, footer: signInFooter,
                onSignInWithChatGPT: { Task { await accounts.startChatGPTLogin() } },
                onSetKey: { provider, key in Task { await accounts.setAPIKey(key, for: provider) }
                },
                onClear: { provider in Task { await accounts.clear(provider) } })
            DisclosureGroup("Add web search (optional)", isExpanded: $isWebSearchOpen) {
                IntegrationsPanel(
                    integrations: accounts.integrations, failure: accounts.integrationError,
                    onSetKey: { integration, key in
                        Task { await accounts.setKey(key, for: integration) }
                    },
                    onClear: { integration in Task { await accounts.clear(integration) } }
                )
                .padding(.top, Theme.Spacing.sm)
            }
            .font(Theme.callout)
        }
    }

    private var signInFooter: String {
        store.accounts.hasSignedInAccount
            ? "You can add or change these later in Settings."
            : "Sign in with at least one of these to continue. "
                + "Web search below is optional and doesn't count."
    }

    private var nameAndFolder: some View {
        @Bindable var store = store
        return Form {
            TextField("Name", text: $store.name, prompt: Text("My Office"))
            LabeledContent("Folder") {
                HStack {
                    Text(store.path).lineLimit(1).truncationMode(.middle)
                    Button("Choose…") { isChoosingFolder = true }
                }
            }
        }
        .formStyle(.grouped)
        .scrollDisabled(true)
        .fixedSize(horizontal: false, vertical: true)
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
                    .keyboardShortcut(.defaultAction)
                    .disabled(!store.canFinish)
            } else {
                Button("Continue") { store.next() }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(!store.canContinue)
            }
        }
    }
}
