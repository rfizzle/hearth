// FollowsLatest makes a scroll view keep its newest row in view while the
// reader is at the bottom and leave them alone once they scroll up, with a
// JumpToLatestButton floating at the bottom edge to get back. A row arriving,
// or a run folding into its trace, changes the content's size; that is never
// the reader scrolling (FollowRule). The button fades with the quick token and
// scrolls with the move token.
// See docs/design.md#the-transcript and docs/design.md#motion

import HearthDesign
import HearthUI
import SwiftUI

/// What the follow rule reads from the scroll view.
private struct ScrollMetrics: Equatable {
    let content: CGFloat
    let viewport: CGFloat
    let distance: CGFloat
}

struct FollowsLatest: ViewModifier {
    /// Whether the bot is still writing, which widens the button to say so.
    let isWriting: Bool

    @State private var following = true
    @State private var position = ScrollPosition(edge: .bottom)
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func body(content: Content) -> some View {
        content
            .scrollPosition($position)
            .defaultScrollAnchor(.bottom)
            .defaultScrollAnchor(following ? .bottom : nil, for: .sizeChanges)
            .onScrollGeometryChange(for: ScrollMetrics.self) { geometry in
                ScrollMetrics(
                    content: geometry.contentSize.height,
                    viewport: geometry.containerSize.height,
                    distance: geometry.contentSize.height - geometry.containerSize.height
                        - geometry.contentOffset.y + geometry.contentInsets.bottom)
            } action: { old, new in
                following = FollowRule.follows(
                    wasFollowing: following, distanceFromBottom: new.distance,
                    contentChanged: old.content != new.content || old.viewport != new.viewport)
            }
            .overlay(alignment: .bottom) {
                if !following {
                    JumpToLatestButton(isWriting: isWriting) {
                        withAnimation(Theme.motion(Theme.move, reduceMotion: reduceMotion)) {
                            position.scrollTo(edge: .bottom)
                        }
                    }
                    .padding(.bottom, Theme.Spacing.sm)
                    .transition(.opacity)
                }
            }
            .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: following)
    }
}
