// SignInPanel is where the owner gives hearth a model to think with: a stack
// of provider cards, one per account, each with its state and its own action
// inline. It is a stack and not a grid because at the column's width a card
// in a grid is too narrow for a name, a pill and a key field with two buttons,
// and a stack gives one linear Tab and VoiceOver order. Only one card has its
// key field open at a time. The panel draws what it is given and calls back;
// the ChatGPT page is opened here, through the openURL environment action,
// when the owner's store publishes it.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// The sign-in surface: a card for ChatGPT and for each key provider.
public struct SignInPanel: View {
    let accounts: [Hearth_V1_ProviderAccount]
    let loginURL: URL?
    let failure: String?
    let footer: String?
    let onSignInWithChatGPT: () -> Void
    let onSetKey: (Hearth_V1_Provider, String) -> Void
    let onClear: (Hearth_V1_Provider) -> Void

    @State private var editing: Hearth_V1_Provider?
    @Environment(\.openURL) private var openURL

    public init(
        accounts: [Hearth_V1_ProviderAccount],
        loginURL: URL?,
        failure: String?,
        footer: String? = nil,
        onSignInWithChatGPT: @escaping () -> Void,
        onSetKey: @escaping (Hearth_V1_Provider, String) -> Void,
        onClear: @escaping (Hearth_V1_Provider) -> Void
    ) {
        self.init(
            accounts: accounts, loginURL: loginURL, failure: failure, footer: footer,
            editing: nil, onSignInWithChatGPT: onSignInWithChatGPT, onSetKey: onSetKey,
            onClear: onClear)
    }

    init(
        accounts: [Hearth_V1_ProviderAccount],
        loginURL: URL?,
        failure: String?,
        footer: String?,
        editing: Hearth_V1_Provider?,
        onSignInWithChatGPT: @escaping () -> Void,
        onSetKey: @escaping (Hearth_V1_Provider, String) -> Void,
        onClear: @escaping (Hearth_V1_Provider) -> Void
    ) {
        self.accounts = accounts
        self.loginURL = loginURL
        self.failure = failure
        self.footer = footer
        self.onSignInWithChatGPT = onSignInWithChatGPT
        self.onSetKey = onSetKey
        self.onClear = onClear
        self._editing = State(initialValue: editing)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            ForEach(accounts, id: \.provider) { account in
                ProviderCard(
                    account: account,
                    failure: editing == account.provider ? failure : nil,
                    isEditing: isEditing(account.provider),
                    onSignIn: onSignInWithChatGPT,
                    onSetKey: { onSetKey(account.provider, $0) },
                    onClear: { onClear(account.provider) })
            }
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
    }

    /// Opening one card's field closes the one that was open.
    private func isEditing(_ provider: Hearth_V1_Provider) -> Binding<Bool> {
        Binding(
            get: { editing == provider },
            set: { open in
                if open {
                    editing = provider
                } else if editing == provider {
                    editing = nil
                }
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
}

extension SignInPanel {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case signedOut
        case signingIn
        case signedIn
        case failed
        case addingKey

        static let failureText = "ChatGPT sign-in did not finish. Try again."

        var accounts: [Hearth_V1_ProviderAccount] {
            switch self {
            case .signedIn:
                Fixtures.accounts
            case .signedOut, .signingIn, .failed, .addingKey:
                [Hearth_V1_Provider.chatgpt, .anthropic, .gemini].map { provider in
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
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            accounts: state.accounts, loginURL: nil, failure: state.failure, footer: nil,
            editing: state.editing,
            onSignInWithChatGPT: {}, onSetKey: { _, _ in }, onClear: { _ in })
    }
}

#Preview("Signed out") { SignInPanel(previewState: .signedOut).padding() }
#Preview("Signing in") { SignInPanel(previewState: .signingIn).padding() }
#Preview("Signed in") { SignInPanel(previewState: .signedIn).padding() }
#Preview("Failed") { SignInPanel(previewState: .failed).padding() }
#Preview("Adding a key") { SignInPanel(previewState: .addingKey).padding() }
