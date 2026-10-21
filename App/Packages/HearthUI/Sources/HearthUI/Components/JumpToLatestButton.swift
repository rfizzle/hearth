// JumpToLatestButton is the round glass button that floats above the composer
// once the reader has scrolled away from the newest message. Pressed, it
// scrolls to the latest row. While the bot is still writing, its down arrow is
// a pulse of three dots so the reader knows work continues without being
// pulled down to it; under Reduce Motion the dots stand still. FollowRule is
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
    let isWriting: Bool
    let action: () -> Void

    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var pulsing = false

    /// A button; `isWriting` swaps the arrow for the pulse.
    public init(isWriting: Bool, action: @escaping () -> Void) {
        self.isWriting = isWriting
        self.action = action
    }

    public var body: some View {
        Button(action: action) {
            Group {
                if isWriting { dots } else { arrow }
            }
            .frame(width: Theme.Size.minTarget, height: Theme.Size.minTarget)
            .contentShape(Circle())
        }
        .buttonStyle(.plain)
        .glassEffect(.regular.interactive(), in: .circle)
        .keyboardShortcut(.downArrow, modifiers: .command)
        .accessibilityLabel(isWriting ? "Jump to latest, still writing" : "Jump to latest")
    }

    private var arrow: some View {
        Image(systemName: "arrow.down")
            .font(Theme.callout.weight(.semibold))
            .foregroundStyle(Theme.color(.textPrimary))
    }

    private var dots: some View {
        HStack(spacing: Theme.Spacing.xs) {
            ForEach(0..<3, id: \.self) { index in
                Circle()
                    .fill(Theme.color(.textPrimary))
                    .frame(width: 5, height: 5)
                    .opacity(pulsing ? 1 : 0.35)
                    .animation(
                        Theme.motion(
                            Theme.move.repeatForever(autoreverses: true)
                                .delay(Double(index) * 0.15), reduceMotion: reduceMotion),
                        value: pulsing)
            }
        }
        .onAppear { pulsing = !reduceMotion }
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
