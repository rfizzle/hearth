// JumpToLatestButton is the round glass button that floats above the composer
// once the reader has scrolled away from the newest message. Pressed, it
// scrolls to the latest row. While the bot is still writing, it widens into a
// capsule that reads "Still writing" beside the arrow, so the reader knows
// work continues without being pulled down to it. FollowRule is
// the rule for when the transcript follows new rows: it does while the bottom
// is in view, stops once the reader scrolls up, and a change in the rows'
// size (a message arriving, a run folding into its trace) never counts as the
// reader scrolling.
// See docs/design.md#materials and docs/design.md#motion

import HearthDesign
import SwiftUI

/// When a transcript keeps its newest row in view.
public enum FollowRule {
    /// How far above the bottom still counts as at the bottom.
    public static let slack: CGFloat = 24

    /// Whether the transcript follows after its scroll geometry changed.
    /// `contentChanged` is true when the content or the viewport changed size;
    /// then the reader did not scroll, so the answer stays as it was. Otherwise
    /// the reader moved, and the transcript follows only at the bottom.
    public static func follows(
        wasFollowing: Bool, distanceFromBottom: CGFloat, contentChanged: Bool
    ) -> Bool {
        if contentChanged { return wasFollowing }
        return distanceFromBottom <= slack
    }
}

/// A round glass button that jumps to the latest message.
public struct JumpToLatestButton: View {
    /// The button's height, and its width while it shows only the arrow.
    static let diameter: CGFloat = Theme.Size.minTarget + Theme.Spacing.sm

    let isWriting: Bool
    let action: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A button; `isWriting` adds "Still writing" beside the arrow.
    public init(isWriting: Bool, action: @escaping () -> Void) {
        self.isWriting = isWriting
        self.action = action
    }

    public var body: some View {
        Button(action: action) {
            HStack(spacing: Theme.Spacing.xs) {
                Image(systemName: "arrow.down")
                    .font(Theme.callout.weight(.semibold))
                    .foregroundStyle(Theme.color(.textPrimary))
                if isWriting {
                    Text("Still writing")
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                        .transition(.opacity)
                }
            }
            .padding(.horizontal, isWriting ? Theme.Spacing.md : 0)
            .frame(minWidth: Self.diameter, minHeight: Self.diameter)
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .glassEffect(.regular.interactive(), in: .capsule)
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isWriting)
        .keyboardShortcut(.downArrow, modifiers: .command)
        .accessibilityLabel(isWriting ? "Jump to latest, still writing" : "Jump to latest")
    }
}

extension JumpToLatestButton {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case scrolledUp
        case scrolledUpWhileWorking
    }

    /// A button showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(isWriting: state == .scrolledUpWhileWorking, action: {})
    }
}

#Preview("Scrolled up") { JumpToLatestButton(previewState: .scrolledUp).padding() }
#Preview("Scrolled up while working") {
    JumpToLatestButton(previewState: .scrolledUpWhileWorking).padding()
}
