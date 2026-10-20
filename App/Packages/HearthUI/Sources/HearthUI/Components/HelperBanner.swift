// HelperBanner is the bar that says hearth's helper stopped and was started
// again, so whatever it was doing has stopped. It sits on the attention fill
// over the top of the detail column, a warning symbol and one sentence in
// callout type, with OK trailing; it stays until the owner dismisses it, and
// it is not a modal. The sentence says what happened and what it cost and
// does not apologise. The symbol travels with the words, so the tint never
// stands alone; the bar reads as one element with OK as its action.
// See docs/model.md#9-failure-handling and docs/design.md#colour

import HearthDesign
import SwiftUI

/// A bar telling the owner the helper restarted, with OK.
public struct HelperBanner: View {
    let onDismiss: () -> Void

    /// What the bar says.
    static let message =
        "Hearth's helper stopped and was restarted. Anything that was running has stopped."

    /// A bar whose OK calls `onDismiss`.
    public init(onDismiss: @escaping () -> Void) {
        self.onDismiss = onDismiss
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.md) {
            Label(Self.message, systemImage: "exclamationmark.triangle")
                .font(Theme.callout)
                .foregroundStyle(Theme.color(Tone.attention))
                .frame(maxWidth: .infinity, alignment: .leading)
            Button("OK", action: onDismiss)
                .accessibilityLabel("Dismiss the helper notice")
        }
        .padding(.horizontal, Theme.Spacing.lg)
        .padding(.vertical, Theme.Spacing.sm)
        .background(Theme.fill(.attention))
        .accessibilityElement(children: .contain)
    }
}

extension HelperBanner {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case restarted
    }

    /// A bar showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(onDismiss: {})
    }
}

#Preview("Restarted") { HelperBanner(previewState: .restarted) }
