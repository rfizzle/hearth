// IntegrationsPanel lists the optional services behind tools, such as web
// search, as the same stack of cards the sign-in panel uses: one card each
// with its mark, what it switches on, a help link and an inline key flow. Only
// one card has its field open at a time. The panel draws what it is given and
// calls back; the same panel is the whole of a Settings pane.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// One card per integration, each with its key.
public struct IntegrationsPanel: View {
    let integrations: [Hearth_V1_IntegrationAccount]
    let failure: String?
    let onSetKey: (Hearth_V1_Integration, String) -> Void
    let onClear: (Hearth_V1_Integration) -> Void

    @State private var editing: Hearth_V1_Integration?

    public init(
        integrations: [Hearth_V1_IntegrationAccount],
        failure: String?,
        onSetKey: @escaping (Hearth_V1_Integration, String) -> Void,
        onClear: @escaping (Hearth_V1_Integration) -> Void
    ) {
        self.init(
            integrations: integrations, failure: failure, editing: nil, onSetKey: onSetKey,
            onClear: onClear)
    }

    init(
        integrations: [Hearth_V1_IntegrationAccount],
        failure: String?,
        editing: Hearth_V1_Integration?,
        onSetKey: @escaping (Hearth_V1_Integration, String) -> Void,
        onClear: @escaping (Hearth_V1_Integration) -> Void
    ) {
        self.integrations = integrations
        self.failure = failure
        self.onSetKey = onSetKey
        self.onClear = onClear
        self._editing = State(initialValue: editing)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            ForEach(integrations, id: \.integration) { account in
                IntegrationCard(
                    account: account,
                    failure: failure,
                    isEditing: isEditing(account.integration),
                    onSetKey: { onSetKey(account.integration, $0) },
                    onClear: { onClear(account.integration) })
            }
        }
    }

    /// Opening one card's field closes the one that was open.
    private func isEditing(_ integration: Hearth_V1_Integration) -> Binding<Bool> {
        Binding(
            get: { editing == integration },
            set: { open in
                if open {
                    editing = integration
                } else if editing == integration {
                    editing = nil
                }
            })
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

        /// The card whose key field is open: a rejected key stays under its field.
        var editing: Hearth_V1_Integration? {
            self == .failed ? .braveSearch : nil
        }
    }

    /// A panel showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            integrations: state.integrations, failure: state.failure, editing: state.editing,
            onSetKey: { _, _ in }, onClear: { _ in })
    }
}

#Preview("Not configured") { IntegrationsPanel(previewState: .notConfigured).padding() }
#Preview("Configured") { IntegrationsPanel(previewState: .configured).padding() }
#Preview("Failed") { IntegrationsPanel(previewState: .failed).padding() }
