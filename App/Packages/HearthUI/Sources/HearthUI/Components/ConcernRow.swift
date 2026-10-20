// ConcernRow is the Chief of Staff's thought about one field of a setup
// sheet's card, drawn under the field it names: a lightbulb in the attention
// tone, the concern in callout type, the suggested replacement on the
// secondary surface when there is one, and two small bordered buttons, Keep
// mine and Use this, or one, Got it, when there is nothing to use. Neither is
// prominent: the sheet's one prominent button is Create. The tone is
// attention because it waits on the person, and the sentence carries it, so
// the colour is never the only sign; it is never the negative tone and never
// an alert, because nothing is wrong. The row is one VoiceOver element,
// "Suggestion for Done when: …", with its buttons inside; the symbol is
// hidden. The words are the core's; this only lays them out.
// See docs/design.md#forms-sheets-and-settings

import HearthDesign
import HearthKit
import SwiftUI

/// A concern about one field, with the owner's two answers to it.
public struct ConcernRow: View {
    let concern: Hearth_V1_Concern
    let field: String
    let onUse: () -> Void
    let onKeep: () -> Void

    /// A row for `concern` about the field labelled `field` ("Done when");
    /// `onUse` takes the suggestion, `onKeep` keeps the owner's words.
    public init(
        concern: Hearth_V1_Concern, field: String, onUse: @escaping () -> Void,
        onKeep: @escaping () -> Void
    ) {
        self.concern = concern
        self.field = field
        self.onUse = onUse
        self.onKeep = onKeep
    }

    public var body: some View {
        HStack(alignment: .top, spacing: Theme.Spacing.sm) {
            Image(systemName: "lightbulb")
                .font(Theme.callout)
                .foregroundStyle(Theme.color(Tone.attention))
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(concern.concern)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .fixedSize(horizontal: false, vertical: true)
                    .accessibilityHidden(true)
                if !concern.suggestion.isEmpty {
                    Text(concern.suggestion)
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textPrimary))
                        .fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(Theme.Spacing.sm)
                        .background(
                            RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                                .fill(Theme.color(.surfaceSecondary))
                        )
                        .accessibilityHidden(true)
                }
                HStack(spacing: Theme.Spacing.sm) {
                    Spacer(minLength: 0)
                    if concern.suggestion.isEmpty {
                        Button("Got it", action: onKeep)
                    } else {
                        Button("Keep mine", action: onKeep)
                        Button("Use this", action: onUse)
                            .accessibilityHint("Puts the suggestion in \(field)")
                    }
                }
                .buttonStyle(.bordered)
                .controlSize(.small)
            }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(Self.spoken(concern, field: field))
    }

    /// What VoiceOver reads for the row: the field, the concern and the
    /// suggestion.
    static func spoken(_ concern: Hearth_V1_Concern, field: String) -> String {
        let said = "Suggestion for \(field): \(concern.concern)"
        return concern.suggestion.isEmpty ? said : "\(said) \(concern.suggestion)"
    }
}

extension ConcernRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case withSuggestion
        case withoutSuggestion
        case longest

        var concern: Hearth_V1_Concern {
            switch self {
            case .withSuggestion: Fixtures.projectDraft.concerns[0]
            case .withoutSuggestion: Fixtures.projectDraft.concerns[1]
            case .longest:
                .with {
                    $0.field = "done_when"
                    $0.concern =
                        "\u{201C}Gone out\u{201D} is hard to check, and nobody will know whether "
                        + "every address on the list was reached. Try a number or a date so the "
                        + "Chief of Staff can tell."
                    $0.suggestion =
                        "The first issue has reached every address on the list, and the second "
                        + "is scheduled for the first Monday of December"
                }
            }
        }

        var field: String {
            self == .withoutSuggestion ? "Name" : "Done when"
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(concern: state.concern, field: state.field, onUse: {}, onKeep: {})
    }
}

#Preview("With a suggestion") {
    ConcernRow(previewState: .withSuggestion).frame(width: 480).padding()
}
#Preview("Without one") { ConcernRow(previewState: .withoutSuggestion).frame(width: 480).padding() }
#Preview("Longest") { ConcernRow(previewState: .longest).frame(width: 480).padding() }
