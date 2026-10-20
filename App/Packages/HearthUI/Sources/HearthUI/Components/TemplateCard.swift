// TemplateCard shows one starting point for a new bot: the role and a line
// about what it does, on a raised card. It is a plain Button around a
// CardSurface, so keyboard and VoiceOver behaviour is the system's; a selected
// card says so in its accessibility traits as well as in its outline and
// check. The same card carries the "Describe the job" choice, a starting point
// with no template behind it, marked by a pencil. It fills the height its row
// gives it, so cards side by side in a grid share one height.
// See docs/design.md#spacing-and-shape

import HearthDesign
import HearthKit
import SwiftUI

/// A role template as a pickable card.
public struct TemplateCard: View {
    let role: String
    let summary: String
    let isSelected: Bool
    let onPick: () -> Void

    let isBlank: Bool

    /// A card with `role` and `summary`; `isBlank` marks the choice with no
    /// template behind it, which shows a pencil.
    public init(
        role: String, summary: String, isSelected: Bool, isBlank: Bool = false,
        onPick: @escaping () -> Void
    ) {
        self.isBlank = isBlank
        self.role = role
        self.summary = summary
        self.isSelected = isSelected
        self.onPick = onPick
    }

    /// A card for `template`.
    public init(template: Hearth_V1_RoleTemplate, isSelected: Bool, onPick: @escaping () -> Void) {
        self.init(
            role: template.role, summary: template.summary, isSelected: isSelected, onPick: onPick)
    }

    public var body: some View {
        Button(action: onPick) {
            CardSurface(isSelected: isSelected) {
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    HStack(alignment: .firstTextBaseline) {
                        Text(role)
                            .font(Theme.headline)
                            .foregroundStyle(Theme.color(.textPrimary))
                        Spacer(minLength: Theme.Spacing.sm)
                        if isSelected {
                            Image(systemName: "checkmark.circle.fill")
                                .foregroundStyle(.tint)
                                .accessibilityHidden(true)
                        } else if isBlank {
                            Image(systemName: "square.and.pencil")
                                .foregroundStyle(Theme.color(.textSecondary))
                                .accessibilityHidden(true)
                        }
                    }
                    Text(summary)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                        .multilineTextAlignment(.leading)
                        .lineLimit(3)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            }
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .frame(maxHeight: .infinity, alignment: .top)
        .accessibilityAddTraits(isSelected ? .isSelected : [])
    }
}

extension TemplateCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case template
        case selected
        case describe

        var role: String {
            self == .describe ? "Describe the job" : Fixtures.templates[0].role
        }

        var summary: String {
            self == .describe
                ? "Start from a blank page and say what you need." : Fixtures.templates[0].summary
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            role: state.role, summary: state.summary, isSelected: state == .selected,
            isBlank: state == .describe, onPick: {})
    }
}

#Preview("Template") { TemplateCard(previewState: .template).padding() }
#Preview("Selected") { TemplateCard(previewState: .selected).padding() }
#Preview("Describe") { TemplateCard(previewState: .describe).padding() }
