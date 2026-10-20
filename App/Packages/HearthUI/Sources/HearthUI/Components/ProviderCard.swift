// ProviderCard is one model provider on a card: its mark, its name, one line
// on what it gives, its state at the top right and the one thing the owner can
// do about it, inline. ChatGPT signs in through the browser; the others take a
// key, which is typed into a field that opens inside the card, so signing in
// reads as choosing a service rather than filling in a form. A service the
// owner added by address is a card of the same shape under the name they gave
// it, saying where it is and which model it answers with, and only Clear. The card is a
// container of system buttons and a SecureField, so keyboard and VoiceOver
// behaviour is theirs; the card itself is one labelled group. It has no hover
// fill and no selected stroke: it holds controls and is not a choice. The key
// field slides in and out as a layout move; the pill and the Add Key and Clear
// swap replace themselves in place.
// See docs/design.md#forms-sheets-and-settings and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// A provider's account as a card with its action inline.
public struct ProviderCard: View {
    let account: Hearth_V1_ProviderAccount
    let failure: String?
    @Binding var isEditing: Bool
    let onSignIn: () -> Void
    let onSetKey: (String) -> Void
    let onClear: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(
        account: Hearth_V1_ProviderAccount,
        failure: String? = nil,
        isEditing: Binding<Bool>,
        onSignIn: @escaping () -> Void,
        onSetKey: @escaping (String) -> Void,
        onClear: @escaping () -> Void
    ) {
        self.account = account
        self.failure = failure
        self._isEditing = isEditing
        self.onSignIn = onSignIn
        self.onSetKey = onSetKey
        self.onClear = onClear
    }

    private var provider: Hearth_V1_Provider { account.provider }
    private var isAddedService: Bool { provider == .openaiCompatible }
    private var isSignedIn: Bool { account.state == .signedIn }
    private var isWaiting: Bool { account.state == .signingIn }
    private var problem: String? {
        account.failure.isEmpty ? failure : account.failure
    }

    public var body: some View {
        CardSurface {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                ServiceMark(named: account.name)
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(account.name)
                            .font(Theme.headline)
                            .foregroundStyle(Theme.color(.textPrimary))
                        Spacer()
                        state
                            .animation(
                                Theme.motion(Theme.quick, reduceMotion: reduceMotion),
                                value: account.state)
                    }
                    Text(account.gives)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                    action
                        .padding(.top, Theme.Spacing.xs)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(account.name), \(stateText)")
        .accessibilityHint(account.gives)
    }

    @ViewBuilder
    private var state: some View {
        if isSignedIn {
            Pill("Signed in", tone: .positive).transition(.blurReplace)
        } else if isWaiting {
            HStack(spacing: Theme.Spacing.sm) {
                ProgressView().controlSize(.small)
                Text("Waiting for the browser…").font(Theme.callout)
            }
            .transition(.blurReplace)
        } else if provider == .chatgpt && problem != nil {
            Pill("Didn't finish", tone: .negative).transition(.blurReplace)
        }
    }

    @ViewBuilder
    private var action: some View {
        if provider == .chatgpt {
            chatGPTAction
        } else if isAddedService {
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                HStack {
                    Text(account.label.isEmpty ? "No key" : "Key ends in …\(account.label)")
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                    Spacer()
                    Button("Clear", action: onClear)
                        .accessibilityLabel("Clear \(account.name)")
                }
                if let problem { FailureLabel(text: problem) }
            }
        } else {
            KeyActions(
                name: provider.displayName,
                keyEnd: isSignedIn ? account.label : nil,
                failure: problem, isEditing: $isEditing,
                onSave: onSetKey, onClear: onClear)
        }
    }

    @ViewBuilder
    private var chatGPTAction: some View {
        if isSignedIn {
            HStack {
                if !account.label.isEmpty {
                    Text("Signed in as \(account.label)")
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                Spacer()
                Button("Sign Out", action: onClear)
            }
        } else if !isWaiting {
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                if let problem { FailureLabel(text: problem) }
                Button(problem == nil ? "Sign in with ChatGPT" : "Try Again", action: onSignIn)
                    .buttonStyle(.borderedProminent)
            }
        }
    }

    private var stateText: String {
        if isSignedIn {
            if account.label.isEmpty { return isAddedService ? "added" : "signed in" }
            return provider == .chatgpt
                ? "signed in as \(account.label)" : "key ends in \(account.label)"
        }
        if isWaiting { return "waiting for the browser" }
        if provider == .chatgpt && problem != nil { return "didn't finish" }
        return "not set up"
    }
}

extension Hearth_V1_ProviderAccount {
    /// The name on the card: the one the owner gave an added service, else
    /// the provider's.
    var name: String {
        provider == .openaiCompatible && !displayName.isEmpty ? displayName : provider.displayName
    }

    /// The one line under the name: where an added service is and the model
    /// it answers with, else what the provider gives.
    var gives: String {
        guard provider == .openaiCompatible else { return provider.gives }
        let host = URL(string: baseURL)?.host() ?? baseURL
        return defaultModel.isEmpty ? host : "\(host) · \(defaultModel)"
    }
}

extension Hearth_V1_Provider {
    /// The one line on what the provider gives the owner.
    var gives: String {
        switch self {
        case .chatgpt: "Uses your ChatGPT plan. Signs in through your browser."
        case .anthropic: "Claude models, with an API key."
        case .gemini: "Google's models, with an API key."
        case .openrouter: "Many models through one key."
        default: "Models, with an API key."
        }
    }
}

/// A failure in words, never colour alone.
struct FailureLabel: View {
    let text: String

    var body: some View {
        Label(text, systemImage: "exclamationmark.triangle")
            .font(Theme.callout)
            .foregroundStyle(Theme.color(.toneNegative))
    }
}

/// The key flow both card kinds share: Add Key opens a field inside the card
/// with Save and Cancel; a stored key shows its last characters and Clear.
/// The field closes when the key is stored, and stays open under a failure.
struct KeyActions: View {
    let name: String
    /// The last characters of the stored key; nil when none is stored.
    let keyEnd: String?
    let failure: String?
    @Binding var isEditing: Bool
    let onSave: (String) -> Void
    let onClear: () -> Void

    private enum Field { case addKey, key }

    @State private var key = ""
    @FocusState private var focus: Field?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            if let keyEnd {
                HStack {
                    Text("Key ends in …\(keyEnd)")
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                    Spacer()
                    Button("Clear", action: onClear)
                        .accessibilityLabel("Clear \(name) key")
                }
                .transition(.blurReplace)
            } else if isEditing {
                field
                if let failure { FailureLabel(text: failure) }
            } else {
                if let failure { FailureLabel(text: failure) }
                Button("Add Key") { setEditing(true) }
                    .focused($focus, equals: .addKey)
                    .transition(.blurReplace)
            }
        }
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isEditing)
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: keyEnd)
        .onChange(of: keyEnd) { _, stored in
            if stored != nil { setEditing(false, refocus: false) }
        }
    }

    private var field: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            SecureField("Paste a key", text: $key)
                .textFieldStyle(.roundedBorder)
                .focused($focus, equals: .key)
                .onSubmit(save)
                .accessibilityLabel("\(name) key")
                .task { focus = .key }
            HStack {
                Spacer()
                Button("Cancel") { setEditing(false) }
                    .keyboardShortcut(.cancelAction)
                Button("Save", action: save)
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .disabled(key.isEmpty)
            }
        }
        .transition(
            .asymmetric(
                insertion: .move(edge: .top).combined(with: .opacity), removal: .opacity))
    }

    private func save() {
        guard !key.isEmpty else { return }
        onSave(key)
    }

    private func setEditing(_ editing: Bool, refocus: Bool = true) {
        isEditing = editing
        if !editing { key = "" }
        if refocus { focus = editing ? .key : .addKey }
    }
}
