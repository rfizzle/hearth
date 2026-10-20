// UnsupportedCard stands in for what this Hearth cannot draw: a widget of a
// kind it does not know, or a whole file of a newer version. It says so in
// words rather than leaving a gap, so a newer file still opens.
// See docs/design.md#empty-states-and-first-run

import HearthDesign
import HearthKit
import SwiftUI

/// A placeholder for a widget or a file made by a newer Hearth.
public struct UnsupportedCard: View {
    let detail: String

    /// A placeholder whose second line is `detail`.
    public init(detail: String) {
        self.detail = detail
    }

    public var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
            Image(systemName: "sparkles.rectangle.stack")
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text("Made by a newer Hearth")
                    .font(Theme.headline)
                    .foregroundStyle(Theme.color(.textPrimary))
                Text(detail)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
        }
        .deliverableCard()
        .accessibilityElement(children: .combine)
    }

    /// The second line for a widget of `kind`.
    nonisolated static func widget(_ kind: String) -> String {
        "A \(kind) widget this Hearth cannot show. Update Hearth to see it."
    }

    /// The second line for a whole file.
    nonisolated static let file =
        "This Hearth cannot show it. Update Hearth, or open it in the browser."
}

extension UnsupportedCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case widget = "a widget"
        case file = "a whole file"
    }

    /// A placeholder showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(detail: state == .widget ? Self.widget("gauge") : Self.file)
    }
}

#Preview("Widget") { UnsupportedCard(previewState: .widget).frame(width: 320).padding() }
