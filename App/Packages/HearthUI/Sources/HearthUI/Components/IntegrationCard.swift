// IntegrationCard is one optional service behind a tool, such as web search,
// on the same card as a provider: its mark, name, what it switches on, a help
// link to where a key comes from, and the inline key flow. Nothing here is
// required, so the card says "Added" when a key is stored and nothing when it
// is not. It is a container of system controls and a Link, so keyboard and
// VoiceOver behaviour is theirs; the card is one labelled group.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// An integration's account as a card with its key flow inline.
public struct IntegrationCard: View {
    let account: Hearth_V1_IntegrationAccount
    let failure: String?
    @Binding var isEditing: Bool
    let onSetKey: (String) -> Void
    let onClear: () -> Void

    public init(
        account: Hearth_V1_IntegrationAccount,
        failure: String? = nil,
        isEditing: Binding<Bool>,
        onSetKey: @escaping (String) -> Void,
        onClear: @escaping () -> Void
    ) {
        self.account = account
        self.failure = failure
        self._isEditing = isEditing
        self.onSetKey = onSetKey
        self.onClear = onClear
    }

    private var summary: String {
        switch account.kind {
        case .webSearch: "Lets your team search the web. Optional."
        default: "Adds a tool to your team. Optional."
        }
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
                        if account.configured { Pill("Added", tone: .positive) }
                    }
                    Text(summary)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                    if !account.helpURL.isEmpty, let help = URL(string: account.helpURL) {
                        Link("Where do I get a key?", destination: help)
                            .font(Theme.callout)
                    }
                    KeyActions(
                        name: account.name, keyEnd: account.configured ? account.label : nil,
                        failure: failure, isEditing: $isEditing,
                        onSave: onSetKey, onClear: onClear
                    )
                    .padding(.top, Theme.Spacing.xs)
                }
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(account.name), \(stateText)")
        .accessibilityHint(summary)
    }

    private var stateText: String {
        guard account.configured else { return "not set up" }
        return account.label.isEmpty ? "added" : "key ends in \(account.label)"
    }
}
