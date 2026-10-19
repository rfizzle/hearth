// IntegrationsPanel lists the optional services behind tools, such as web
// search: one row each with its name, what it switches on, a key field or the
// stored key's last characters with Clear, and a help link to where a key
// comes from. It composes a SecureField, buttons and a Link, so keyboard and
// VoiceOver behaviour is theirs. The panel draws what it is given and calls
// back; the same panel is the whole of a later Settings screen.

import HearthDesign
import HearthKit
import SwiftUI

/// One row per integration, each with its key.
public struct IntegrationsPanel: View {
    let integrations: [Hearth_V1_IntegrationAccount]
    let failure: String?
    let onSetKey: (Hearth_V1_Integration, String) -> Void
    let onClear: (Hearth_V1_Integration) -> Void

    public init(
        integrations: [Hearth_V1_IntegrationAccount],
        failure: String?,
        onSetKey: @escaping (Hearth_V1_Integration, String) -> Void,
        onClear: @escaping (Hearth_V1_Integration) -> Void
    ) {
        self.integrations = integrations
        self.failure = failure
        self.onSetKey = onSetKey
        self.onClear = onClear
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            ForEach(integrations, id: \.integration) { account in
                IntegrationRow(
                    account: account,
                    onSetKey: { onSetKey(account.integration, $0) },
                    onClear: { onClear(account.integration) })
            }
            if let failure {
                Text(failure)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .accessibilityLabel("Problem: \(failure)")
            }
        }
    }
}

private struct IntegrationRow: View {
    let account: Hearth_V1_IntegrationAccount
    let onSetKey: (String) -> Void
    let onClear: () -> Void

    @State private var key = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(account.name)
                .font(.headline)
                .foregroundStyle(Theme.color(.textPrimary))
            Text(summary)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
            if account.configured {
                HStack {
                    Text("Key ends in …\(account.label)")
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textPrimary))
                    Button("Clear", action: onClear)
                        .accessibilityLabel("Clear \(account.name) key")
                }
            } else {
                HStack {
                    SecureField("\(account.name) key", text: $key)
                    Button("Save") {
                        onSetKey(key)
                        key = ""
                    }
                    .disabled(key.isEmpty)
                }
            }
            if !account.helpURL.isEmpty, let help = URL(string: account.helpURL) {
                Link("Where do I get a key?", destination: help)
            }
        }
    }

    private var summary: String {
        switch account.kind {
        case .webSearch: "Lets your team search the web."
        default: "Adds a tool to your team."
        }
    }
}

extension IntegrationsPanel {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case notConfigured
        case configured
        case failed

        var integrations: [Hearth_V1_IntegrationAccount] {
            Fixtures.integrations.map { account in
                var account = account
                if self == .configured {
                    account.configured = true
                    account.label = "1234"
                }
                return account
            }
        }

        var failure: String? {
            self == .failed ? "That key was not accepted. Check it and try again." : nil
        }
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            integrations: state.integrations, failure: state.failure,
            onSetKey: { _, _ in }, onClear: { _ in })
    }
}

#Preview("Not configured") { IntegrationsPanel(previewState: .notConfigured).padding() }
#Preview("Configured") { IntegrationsPanel(previewState: .configured).padding() }
#Preview("Failed") { IntegrationsPanel(previewState: .failed).padding() }
