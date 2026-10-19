// TemplateCard shows one starting point for a new bot: the role and a line
// about what it does. It is a Button, so keyboard and VoiceOver behaviour is
// the system's; a selected card says so in its accessibility traits as well as
// in its outline. The same card carries the "Describe the job" choice, which
// is a starting point with no template behind it.

import HearthDesign
import HearthKit
import SwiftUI

/// A role template as a pickable card.
public struct TemplateCard: View {
    let role: String
    let summary: String
    let isSelected: Bool
    let onPick: () -> Void

    public init(role: String, summary: String, isSelected: Bool, onPick: @escaping () -> Void) {
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
            VStack(alignment: .leading, spacing: 4) {
                Text(role)
                    .font(.headline)
                    .foregroundStyle(Theme.color(.textPrimary))
                Text(summary)
                    .font(Theme.body)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .multilineTextAlignment(.leading)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(12)
            .background(
                RoundedRectangle(cornerRadius: 8)
                    .strokeBorder(
                        isSelected ? Color.accentColor : Theme.color(.textSecondary),
                        lineWidth: isSelected ? 2 : 1))
        }
        .buttonStyle(.plain)
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
            role: state.role, summary: state.summary, isSelected: state == .selected, onPick: {})
    }
}

#Preview("Template") { TemplateCard(previewState: .template).padding() }
#Preview("Selected") { TemplateCard(previewState: .selected).padding() }
#Preview("Describe") { TemplateCard(previewState: .describe).padding() }
