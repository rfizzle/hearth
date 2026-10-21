// ShowsProduced puts the ProducedCard on a transcript when the conversation has
// produced a file or a deliverable. With room beside the readable column the
// card floats at the top trailing edge; on a narrower window it sits above the
// transcript, so it never covers a message or the jump button at the bottom.
// Whether the card is folded is remembered per conversation in the viewer's
// defaults.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

struct ShowsProduced: ViewModifier {
    let list: ProducedList
    @AppStorage private var collapsed: Bool
    @State private var width: CGFloat = .infinity
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    init(messages: [Hearth_V1_Message], conversationID: String) {
        list = ProducedList(messages: messages)
        _collapsed = AppStorage(wrappedValue: false, "producedCollapsed.\(conversationID)")
    }

    /// The width a window needs for the card to float beside the column.
    private static let roomNeeded =
        Theme.Size.readableWidth + 2 * (ProducedCard.width + 2 * Theme.Spacing.lg)

    func body(content: Content) -> some View {
        let beside = width >= Self.roomNeeded
        content
            .onGeometryChange(for: CGFloat.self, of: \.size.width) { width = $0 }
            .overlay(alignment: .topTrailing) {
                if beside, !list.isEmpty { card.padding(Theme.Spacing.lg) }
            }
            .safeAreaInset(edge: .top, spacing: 0) {
                if !beside, !list.isEmpty {
                    card
                        .padding(.horizontal, Theme.Spacing.lg)
                        .padding(.top, Theme.Spacing.sm)
                        .frame(maxWidth: Theme.Size.readableWidth, alignment: .trailing)
                        .frame(maxWidth: .infinity)
                }
            }
            .animation(
                Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: list.isEmpty)
    }

    private var card: some View {
        ProducedCard(list: list, isCollapsed: $collapsed)
            .transition(.opacity)
    }
}
