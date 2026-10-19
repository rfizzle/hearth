// SignInPanel is where the owner gives hearth a model to think with: a Sign in
// with ChatGPT button, a provider picker with a key field, and one row per
// account with its state. It composes system buttons, a Picker and a
// SecureField, so keyboard and VoiceOver behaviour is theirs; each account row
// reads as one element. The panel draws what it is given and calls back; the
// ChatGPT page is opened here, through the openURL environment action, when
// the owner's store publishes it.

import HearthDesign
import HearthKit
import SwiftUI

/// The sign-in surface: ChatGPT, API keys and the accounts that exist.
public struct SignInPanel: View {
    let accounts: [Hearth_V1_ProviderAccount]
    let loginURL: URL?
    let failure: String?
    let onSignInWithChatGPT: () -> Void
    let onSetKey: (Hearth_V1_Provider, String) -> Void
    let onClear: (Hearth_V1_Provider) -> Void

    @State private var provider: Hearth_V1_Provider = .anthropic
    @State private var key = ""
    @Environment(\.openURL) private var openURL

    /// The providers that sign in with a key.
    static let keyProviders: [Hearth_V1_Provider] = [.anthropic, .gemini, .openrouter]

    public init(
        accounts: [Hearth_V1_ProviderAccount],
        loginURL: URL?,
        failure: String?,
        onSignInWithChatGPT: @escaping () -> Void,
        onSetKey: @escaping (Hearth_V1_Provider, String) -> Void,
        onClear: @escaping (Hearth_V1_Provider) -> Void
    ) {
        self.accounts = accounts
        self.loginURL = loginURL
        self.failure = failure
        self.onSignInWithChatGPT = onSignInWithChatGPT
        self.onSetKey = onSetKey
        self.onClear = onClear
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Button("Sign in with ChatGPT", action: onSignInWithChatGPT)
                .buttonStyle(.borderedProminent)
            HStack {
                Picker("Provider", selection: $provider) {
                    ForEach(Self.keyProviders, id: \.self) { Text($0.displayName).tag($0) }
                }
                SecureField("API key", text: $key)
                Button("Save") {
                    onSetKey(provider, key)
                    key = ""
                }
                .disabled(key.isEmpty)
            }
            ForEach(accounts, id: \.provider) { account in
                AccountRow(account: account, onClear: { onClear(account.provider) })
            }
            if let failure {
                Text(failure)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .accessibilityLabel("Sign-in problem: \(failure)")
            }
        }
        .onChange(of: loginURL) { _, url in
            if let url { openURL(url) }
        }
    }
}

private struct AccountRow: View {
    let account: Hearth_V1_ProviderAccount
    let onClear: () -> Void

    var body: some View {
        HStack {
            Text(account.provider.displayName)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
            Spacer()
            Text(status)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
            if account.state == .signedIn {
                Button("Clear", action: onClear)
                    .accessibilityLabel("Clear \(account.provider.displayName)")
            }
        }
        .accessibilityElement(children: .contain)
    }

    private var status: String {
        switch account.state {
        case .signedIn:
            if account.label.isEmpty {
                "Signed in"
            } else if account.provider == .chatgpt {
                "Signed in as \(account.label)"
            } else {
                "Key ends in …\(account.label)"
            }
        case .signingIn:
            "Waiting for the browser…"
        default:
            "Signed out"
        }
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

        var accounts: [Hearth_V1_ProviderAccount] {
            switch self {
            case .signedIn:
                Fixtures.accounts
            case .signedOut, .signingIn, .failed:
                [Hearth_V1_Provider.chatgpt, .anthropic, .gemini].map { provider in
                    .with {
                        $0.provider = provider
                        $0.state =
                            self == .signingIn && provider == .chatgpt ? .signingIn : .signedOut
                    }
                }
            }
        }

        var failure: String? {
            self == .failed ? "ChatGPT sign-in did not finish. Try again." : nil
        }
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            accounts: state.accounts, loginURL: nil, failure: state.failure,
            onSignInWithChatGPT: {}, onSetKey: { _, _ in }, onClear: { _ in })
    }
}

#Preview("Signed out") { SignInPanel(previewState: .signedOut).padding() }
#Preview("Signing in") { SignInPanel(previewState: .signingIn).padding() }
#Preview("Signed in") { SignInPanel(previewState: .signedIn).padding() }
#Preview("Failed") { SignInPanel(previewState: .failed).padding() }
