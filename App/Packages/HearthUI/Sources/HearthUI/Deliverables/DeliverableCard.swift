// The pieces every deliverable widget shares: the card it sits on, filling the
// height its grid row gives it so cards side by side share one height, and the
// line that says where a bound figure came from. A live figure carries a quiet
// "Live" pill; a written one says when; a missing one says why in the
// attention tone, the word carrying what the colour supports.
// See docs/design.md#colour

import HearthDesign
import SwiftUI

extension View {
    /// Sets a deliverable widget on a card that fills its grid cell.
    func deliverableCard() -> some View {
        frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .cardSurface()
    }
}

/// A widget's title, read as a heading.
struct WidgetTitle: View {
    let text: String?

    var body: some View {
        if let text, !text.isEmpty {
            Text(text)
                .font(Theme.headline)
                .foregroundStyle(Theme.color(.textPrimary))
                .accessibilityAddTraits(.isHeader)
        }
    }
}

/// Where a bound figure came from, or nothing for an unbound one.
struct BoundLabel: View {
    let bound: BoundValue

    var body: some View {
        switch bound {
        case .unbound:
            EmptyView()
        case .live:
            Pill("Live")
        case .written, .missing:
            if let label = bound.label() {
                Text(label)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(bound.tone))
            }
        }
    }
}

/// A caption under a figure, in secondary text.
struct CaptionLine: View {
    let text: String?

    var body: some View {
        if let text, !text.isEmpty {
            Text(text)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
        }
    }
}
