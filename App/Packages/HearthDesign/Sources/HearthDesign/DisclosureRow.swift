// DisclosureRow is a quiet row that opens to show more: a tool call under a
// bot's turn. It wraps DisclosureGroup, so the arrow, the click target, the
// keyboard (Space and the arrow keys) and the VoiceOver expanded and collapsed
// state are the system's. This only sets the label in secondary callout type,
// keeps it at least a minimum target tall, and insets the content. Opening
// and closing moves the height with the layout move while the body fades in,
// and the container moves its children as one so siblings slide rather than
// jump.
// See docs/design.md#the-transcript and docs/design.md#motion

import SwiftUI

/// A row that discloses its content.
public struct DisclosureRow<Label: View, Content: View>: View {
    @Binding var isExpanded: Bool
    let label: Label
    let content: Content

    @ScaledMetric(relativeTo: .callout) private var minHeight: CGFloat = Theme.Size.minTarget
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(
        isExpanded: Binding<Bool>,
        @ViewBuilder label: () -> Label,
        @ViewBuilder content: () -> Content
    ) {
        self._isExpanded = isExpanded
        self.label = label()
        self.content = content()
    }

    public var body: some View {
        DisclosureGroup(isExpanded: $isExpanded) {
            content
                .padding(.leading, Theme.Spacing.lg)
                .transition(.opacity)
        } label: {
            label
                .font(Theme.callout)
                .foregroundStyle(Theme.color(.textSecondary))
                .frame(minHeight: minHeight, alignment: .leading)
        }
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isExpanded)
        .geometryGroup()
    }
}

extension DisclosureRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case collapsed
        case expanded

        /// Whether the row is open in this state.
        public var isExpanded: Bool { self == .expanded }
    }
}

extension DisclosureRow where Label == Text, Content == Text {
    /// A row with sample text in `state`, for the gallery and previews. The
    /// binding is fixed, so the row does not respond to clicks.
    public init(previewState state: PreviewState) {
        self.init(
            isExpanded: .constant(state.isExpanded),
            label: { Text("Searched the web") },
            content: { Text("3 results for “quarterly filing dates”") }
        )
    }
}

#Preview("Collapsed") { DisclosureRow(previewState: .collapsed).padding() }
#Preview("Expanded") { DisclosureRow(previewState: .expanded).padding() }
