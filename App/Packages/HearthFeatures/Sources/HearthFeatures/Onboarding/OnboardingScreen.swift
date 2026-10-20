// OnboardingScreen shows the store's step in a centred column: a welcome, the
// sign-in cards with the optional web-search card folded away under them, and
// the name and folder form. Optional setup waits behind a collapsed
// disclosure so the owner is not asked for it before they can continue. The
// folder is picked with the system file importer limited to directories, so
// the owner sees the standard sheet and keyboard access is the system's. The
// screen is a function of store state; its only local state is whether the
// importer is open and whether the web-search disclosure is. A step change
// cross-fades while the column's height moves with it.
// See docs/design.md#empty-states-and-first-run and docs/design.md#motion

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
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @ScaledMetric(relativeTo: .largeTitle) private var markSize: CGFloat = 72
    @ScaledMetric(relativeTo: .title) private var iconColumn: CGFloat = 36

    public init(store: OnboardingStore) {
        self.store = store
    }

    public var body: some View {
        GeometryReader { proxy in
            ScrollView {
                content
                    .frame(maxWidth: Theme.Size.formWidth)
                    .frame(maxWidth: .infinity, minHeight: proxy.size.height)
            }
            .scrollBounceBehavior(.basedOnSize)
        }
        .padding(Theme.Spacing.xxl)
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(.background)
        .navigationTitle("Welcome")
        .task { await store.accounts.load() }
    }

    /// The step's content, centred in the window; the buttons follow it so
    /// they never float alone at the window's edge.
    private var content: some View {
        VStack(alignment: store.step == .welcome ? .center : .leading, spacing: Theme.Spacing.xl) {
            header
            switch store.step {
            case .welcome: outline.transition(.opacity)
            case .signIn: signIn.transition(.opacity)
            case .nameAndFolder: nameAndFolder.transition(.opacity)
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.toneNegative))
            }
            buttons
        }
        .padding(.vertical, Theme.Spacing.lg)
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: store.step)
        .geometryGroup()
    }

    private var header: some View {
        let welcome = store.step == .welcome
        return VStack(
            alignment: welcome ? .center : .leading,
            spacing: welcome ? Theme.Spacing.md : Theme.Spacing.xs
        ) {
            if welcome {
                Image(systemName: "flame.fill")
                    .font(.system(size: markSize))
                    .foregroundStyle(.tint)
                    .padding(.bottom, Theme.Spacing.lg)
                    .accessibilityHidden(true)
            }
            Text("Step \(store.step.rawValue + 1) of \(Self.stepCount)")
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            Text(heading)
                .font(welcome ? Theme.largeTitle : Theme.title)
                .multilineTextAlignment(welcome ? .center : .leading)
                .foregroundStyle(Theme.color(.textPrimary))
                .accessibilityAddTraits(.isHeader)
            Text(lead)
                .font(welcome ? Theme.title : Theme.body)
                .fontWeight(.regular)
                .multilineTextAlignment(welcome ? .center : .leading)
                .foregroundStyle(Theme.color(.textSecondary))
        }
    }

    /// The three steps ahead, one short line each.
    private var outline: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
            ForEach(Self.outlineLines, id: \.text) { line in
                Label {
                    Text(line.text)
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textPrimary))
                } icon: {
                    Image(systemName: line.symbol)
                        .font(Theme.title)
                        .foregroundStyle(.tint)
                        .frame(width: iconColumn)
                }
            }
        }
    }

    private static let outlineLines: [(symbol: String, text: String)] = [
        ("person.crop.circle", "Sign in"),
        ("building.2", "Name your office"),
        ("folder", "Choose where work lands"),
    ]

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
                accounts: accounts.accounts, knownServices: accounts.knownServices,
                loginURL: accounts.loginURL, failure: accounts.signInFailure,
                serviceFailure: accounts.serviceError, footer: signInFooter,
                featured: [.chatgpt, .openrouter],
                onSignInWithChatGPT: { Task { await accounts.startChatGPTLogin() } },
                onSetKey: { provider, key in Task { await accounts.setAPIKey(key, for: provider) }
                },
                onClear: { provider in Task { await accounts.clear(provider) } },
                onSaveService: { request in Task { await accounts.saveService(request) } },
                onRemoveService: { profile in Task { await accounts.removeService(profile) } },
                onCloseService: { accounts.dismissServiceError() })
            DisclosureGroup("Add web search (optional)", isExpanded: $isWebSearchOpen) {
                IntegrationsPanel(
                    integrations: accounts.integrations, failure: accounts.integrationError,
                    onSetKey: { integration, key in
                        Task { await accounts.setKey(key, for: integration) }
                    },
                    onClear: { integration in Task { await accounts.clear(integration) } }
                )
                .padding(.top, Theme.Spacing.sm)
                .transition(.opacity)
            }
            .font(Theme.callout)
            .animation(
                Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isWebSearchOpen)
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

    /// Welcome has one centred, large Continue; later steps put Back leading
    /// and Continue or Done trailing on one row under the content.
    private var buttons: some View {
        HStack {
            if store.step != .welcome {
                Button("Back") { store.back() }
                Spacer()
            }
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
        .controlSize(store.step == .welcome ? .large : .regular)
        .frame(maxWidth: .infinity, alignment: store.step == .welcome ? .center : .trailing)
    }
}
