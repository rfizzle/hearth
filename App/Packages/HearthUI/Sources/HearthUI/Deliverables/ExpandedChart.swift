// ExpandedChart shows one chart large over its window: the chart on a card
// above a dimmed backdrop, with the same hover callouts, a menu for its data
// and image, and a close button that Escape also presses. It grows out of the
// card it came from, sharing the card's frame through the window's namespace,
// and fades in instead under Reduce Motion. The window hosts it and drives the
// change with `ExpandedChart.animation`.
// See docs/design.md#motion

import HearthDesign
import SwiftUI

/// The buttons on a chart card the keyboard can reach.
enum ChartControl: Hashable {
    case expand, more, close
}

/// Marks a card as the place an expanded chart zooms from.
struct ZoomSource: ViewModifier {
    let id: UUID
    let zoom: ChartZoom?

    func body(content: Content) -> some View {
        if let zoom {
            content.matchedGeometryEffect(id: id, in: zoom.namespace, isSource: true)
        } else {
            content
        }
    }
}

/// One chart, large, over a dimmed backdrop.
public struct ExpandedChart: View {
    let id: UUID
    let shape: ChartCard.Shape
    let namespace: Namespace.ID
    let onClose: @MainActor () -> Void
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// The chart `shape` that opened from the card `id` in `namespace`;
    /// `onClose` runs for the close button, Escape and a click on the backdrop.
    public init(
        id: UUID, shape: ChartCard.Shape, namespace: Namespace.ID,
        onClose: @escaping @MainActor () -> Void
    ) {
        self.id = id
        self.shape = shape
        self.namespace = namespace
        self.onClose = onClose
    }

    /// The animation for opening and closing: a layout move, or a quick fade
    /// under Reduce Motion, where the zoom would be a jump.
    public static func animation(reduceMotion: Bool) -> Animation {
        reduceMotion ? Theme.quick : Theme.move
    }

    public var body: some View {
        ZStack {
            Color.black.opacity(0.35)
                .ignoresSafeArea()
                .contentShape(Rectangle())
                .onTapGesture(perform: onClose)
                .accessibilityHidden(true)
            panel
                .padding(Theme.Spacing.xl)
                .frame(maxWidth: 1000)
        }
        .transition(.opacity)
        .onExitCommand(perform: onClose)
    }

    @ViewBuilder private var panel: some View {
        let card = ChartCard(shape, mode: .expanded(close: onClose))
            .shadow(color: .black.opacity(0.2), radius: 24, y: 8)
        if reduceMotion {
            card
        } else {
            card.matchedGeometryEffect(id: id, in: namespace, isSource: false)
        }
    }
}

extension ExpandedChart {
    /// A large chart for the gallery and previews: the line, the pie.
    public init(previewState state: ChartCard.PreviewState, namespace: Namespace.ID) {
        self.init(id: UUID(), shape: state.shape, namespace: namespace, onClose: {})
    }
}
