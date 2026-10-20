// CardSurface sets its content on a raised card: the card colour on a
// continuous rounded rectangle with a hairline separator stroke, or the
// accent colour in a heavier stroke when selected. It is a plain shape, not a
// control, so it adds nothing for VoiceOver: a card that can be chosen is a
// Button or a selectable row around it, which carries the traits. The
// selected state is shown by the stroke and must also be said by whatever
// wraps it, because a stroke is not a label.

import SwiftUI

/// Content on a raised, rounded card.
public struct CardSurface<Content: View>: View {
    let isSelected: Bool
    let content: Content

    public init(isSelected: Bool = false, @ViewBuilder content: () -> Content) {
        self.isSelected = isSelected
        self.content = content()
    }

    public var body: some View {
        content.cardSurface(isSelected: isSelected)
    }
}

extension View {
    /// Sets this view on a card; `isSelected` swaps the hairline for the accent.
    public func cardSurface(isSelected: Bool = false) -> some View {
        modifier(CardSurfaceModifier(isSelected: isSelected))
    }
}

private struct CardSurfaceModifier: ViewModifier {
    let isSelected: Bool

    func body(content: Content) -> some View {
        let shape = RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
        content
            .padding(Theme.Spacing.md)
            .background(shape.fill(Theme.color(.surfaceCard)))
            .overlay {
                if isSelected {
                    shape.strokeBorder(.tint, lineWidth: 2)
                } else {
                    shape.strokeBorder(.separator, lineWidth: 1)
                }
            }
    }
}

extension CardSurface {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case plain
        case selected

        /// Whether the card is selected in this state.
        public var isSelected: Bool { self == .selected }
    }
}

extension CardSurface where Content == Text {
    /// A card holding sample text in `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(isSelected: state.isSelected) { Text("Researcher") }
    }
}

#Preview("Plain") { CardSurface(previewState: .plain).padding() }
#Preview("Selected") { CardSurface(previewState: .selected).padding() }
