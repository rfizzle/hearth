// SignInPanel is where the owner gives hearth a model to think with: a stack
// of provider cards, one per account, each with its state and its own action
// inline, then a card for a service hearth has no card for. It is a stack and
// not a grid because at the column's width a card in a grid is too narrow for
// a name, a pill and a key field with two buttons, and a stack gives one
// linear Tab and VoiceOver order. It composes ProviderCard, one per built-in
// provider and one per service the owner added, and ServicePicker. Only one
// card is open at a time: a key field, or the picker's list or fields. In
// onboarding every built-in provider has a card and the picker is "Another
// service"; in Settings only the configured ones do and the picker is "Add a
// service", whose list then offers the built-ins too, so choosing one shows
// its card. The panel draws what it is given and calls back; the ChatGPT page
// is opened here, through the openURL environment action, when the owner's
// store publishes it.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// The sign-in surface: a card for ChatGPT, for each key provider, for each
/// added service, and one to add another.
public struct SignInPanel: View {
    let accounts: [Hearth_V1_ProviderAccount]
    let knownServices: [Hearth_V1_KnownService]
    let loginURL: URL?
    let failure: String?
    let serviceFailure: String?
    let footer: String?
    /// The built-ins that keep a card while signed out; the rest wait in
    /// the picker so a first screen stays short.
    let featured: Set<Hearth_V1_Provider>
    let onSignInWithChatGPT: () -> Void
    let onSetKey: (Hearth_V1_Provider, String) -> Void
    let onClear: (Hearth_V1_Provider) -> Void
    let onSaveService: (Hearth_V1_SetAPIKeyRequest) -> Void
    let onRemoveService: (String) -> Void
    let onCloseService: () -> Void
    let onListModels: ModelLister?

    @State private var editing: Hearth_V1_Provider?
    @State private var adding: ServicePicker.Stage?
    /// Built-in cards chosen from the picker in Settings, shown although
    /// signed out.
    @State private var revealed: Set<Hearth_V1_Provider> = []
    @Environment(\.openURL) private var openURL

    /// A panel over `accounts`. A signed-out built-in outside `featured` has
    /// no card until it is chosen from the picker: onboarding features ChatGPT
    /// and OpenRouter, Settings features none.
    /// `onCloseService` is told when the picker closes, so a failure shown in
    /// it can be forgotten.
    public init(
        accounts: [Hearth_V1_ProviderAccount],
        knownServices: [Hearth_V1_KnownService],
        loginURL: URL?,
        failure: String?,
        serviceFailure: String? = nil,
        footer: String? = nil,
        featured: Set<Hearth_V1_Provider> = [.chatgpt, .anthropic, .gemini, .openrouter],
        onSignInWithChatGPT: @escaping () -> Void,
        onSetKey: @escaping (Hearth_V1_Provider, String) -> Void,
        onClear: @escaping (Hearth_V1_Provider) -> Void,
        onSaveService: @escaping (Hearth_V1_SetAPIKeyRequest) -> Void,
        onRemoveService: @escaping (String) -> Void,
        onCloseService: @escaping () -> Void,
        onListModels: ModelLister? = nil
    ) {
        self.onListModels = onListModels
        self.accounts = accounts
        self.knownServices = knownServices
        self.loginURL = loginURL
        self.failure = failure
        self.serviceFailure = serviceFailure
        self.footer = footer
        self.featured = featured
        self.onSignInWithChatGPT = onSignInWithChatGPT
        self.onSetKey = onSetKey
        self.onClear = onClear
        self.onSaveService = onSaveService
        self.onRemoveService = onRemoveService
        self.onCloseService = onCloseService
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            ForEach(builtins, id: \.provider) { account in
                ProviderCard(
                    account: account,
                    failure: editing == account.provider ? failure : nil,
                    isEditing: isEditing(account.provider),
                    onSignIn: onSignInWithChatGPT,
                    onSetKey: { onSetKey(account.provider, $0) },
                    onClear: { onClear(account.provider) })
            }
            ForEach(services, id: \.profile) { account in
                ProviderCard(
                    account: account, isEditing: .constant(false),
                    onSignIn: {}, onSetKey: { _ in },
                    onClear: { onRemoveService(account.profile) })
            }
            ServicePicker(
                title: featured.isEmpty ? "Add a service" : "Another service",
                services: offered, failure: serviceFailure, stage: addingStage,
                onChooseBuiltin: choose, onSave: onSaveService, onListModels: onListModels)
            if let failure, editing == nil, !accounts.contains(where: { !$0.failure.isEmpty }) {
                FailureLabel(text: failure)
            }
            if let footer {
                Text(footer)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
        }
        .onChange(of: loginURL) { _, url in
            if let url { openURL(url) }
        }
        .onChange(of: services.map(\.profile)) { old, new in
            // A service the owner was adding is now a card of its own.
            if new.count > old.count, case .fields = adding { adding = nil }
        }
        .onChange(of: adding) { _, stage in
            if stage == nil { onCloseService() }
        }
    }

    /// The built-in cards shown: the featured ones, the configured ones, and
    /// any the owner is setting up from the picker.
    private var builtins: [Hearth_V1_ProviderAccount] {
        accounts.filter { account in
            account.provider != .openaiCompatible
                && (featured.contains(account.provider) || account.state != .signedOut
                    || revealed.contains(account.provider) || editing == account.provider)
        }
    }

    private var services: [Hearth_V1_ProviderAccount] {
        accounts.filter { $0.provider == .openaiCompatible }
    }

    /// The picker's rows: every known service but a built-in whose card is
    /// already on screen.
    private var offered: [Hearth_V1_KnownService] {
        let shown = Set(builtins.map(\.provider.knownServiceID))
        return knownServices.filter { $0.kind != .builtin || !shown.contains($0.id) }
    }

    /// A built-in chosen from the picker gets its card, with the key field
    /// open when it takes a key.
    private func choose(_ service: Hearth_V1_KnownService) {
        guard let account = accounts.first(where: { $0.provider.knownServiceID == service.id })
        else { return }
        revealed.insert(account.provider)
        if account.provider != .chatgpt { editing = account.provider }
    }

    /// Opening one card's field closes the one that was open, and the picker.
    private func isEditing(_ provider: Hearth_V1_Provider) -> Binding<Bool> {
        Binding(
            get: { editing == provider },
            set: { open in
                if open {
                    editing = provider
                    adding = nil
                } else if editing == provider {
                    editing = nil
                }
            })
    }

    /// Opening the picker closes any key field.
    private var addingStage: Binding<ServicePicker.Stage?> {
        Binding(
            get: { adding },
            set: { stage in
                adding = stage
                if stage != nil { editing = nil }
            })
    }
}

extension Hearth_V1_Provider {
    /// The name the owner knows the provider by.
    var displayName: String {
        switch self {
        case .chatgpt: "ChatGPT"
        case .anthropic: "Anthropic"
        case .gemini: "Gemini"
        case .openrouter: "OpenRouter"
        case .openaiCompatible: "Compatible endpoint"
        default: "Provider"
        }
    }

    /// The id the core's known-services list gives this provider's row.
    var knownServiceID: String {
        switch self {
        case .chatgpt: "chatgpt"
        case .anthropic: "anthropic"
        case .gemini: "gemini"
        case .openrouter: "openrouter"
        default: ""
        }
    }
}

extension SignInPanel {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case signedOut
        case signingIn
        case signedIn
        case failed
        case addingKey
        case serviceList
        case serviceFields
        case addedService
        case settings

        static let failureText = "ChatGPT sign-in did not finish. Try again."

        var accounts: [Hearth_V1_ProviderAccount] {
            switch self {
            case .signedIn:
                Fixtures.accounts
            case .addedService, .settings:
                Fixtures.accounts + [Fixtures.customService]
            case .signedOut, .signingIn, .failed, .addingKey, .serviceList, .serviceFields:
                [Hearth_V1_Provider.chatgpt, .anthropic, .gemini, .openrouter].map { provider in
                    .with {
                        $0.provider = provider
                        $0.state =
                            self == .signingIn && provider == .chatgpt ? .signingIn : .signedOut
                        if self == .failed && provider == .chatgpt { $0.failure = Self.failureText }
                    }
                }
            }
        }

        var failure: String? {
            self == .failed ? Self.failureText : nil
        }

        /// The card whose key field is open.
        var editing: Hearth_V1_Provider? {
            self == .addingKey ? .anthropic : nil
        }

        /// How far the picker is open.
        var adding: ServicePicker.Stage? {
            switch self {
            case .serviceList: .list
            case .serviceFields: .fields(nil)
            default: nil
            }
        }
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            accounts: state.accounts, knownServices: Fixtures.knownServices, loginURL: nil,
            failure: state.failure, featured: state == .settings ? [] : [.chatgpt, .openrouter],
            onSignInWithChatGPT: {}, onSetKey: { _, _ in }, onClear: { _ in },
            onSaveService: { _ in }, onRemoveService: { _ in }, onCloseService: {},
            onListModels: { _, _ in Fixtures.ollamaModels })
        _editing = State(initialValue: state.editing)
        _adding = State(initialValue: state.adding)
    }
}

#Preview("Signed out") { SignInPanel(previewState: .signedOut).padding() }
#Preview("Signing in") { SignInPanel(previewState: .signingIn).padding() }
#Preview("Signed in") { SignInPanel(previewState: .signedIn).padding() }
#Preview("Failed") { SignInPanel(previewState: .failed).padding() }
#Preview("Adding a key") { SignInPanel(previewState: .addingKey).padding() }
#Preview("Service list") { SignInPanel(previewState: .serviceList).padding() }
#Preview("Service fields") { SignInPanel(previewState: .serviceFields).padding() }
#Preview("Added service") { SignInPanel(previewState: .addedService).padding() }
#Preview("Settings") { SignInPanel(previewState: .settings).padding() }
