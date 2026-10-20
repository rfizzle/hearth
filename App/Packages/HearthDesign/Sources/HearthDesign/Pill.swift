// Pill is a short label on a tinted capsule: a role, a count, a state. It wraps
// a Text on a Capsule, so it grows with Dynamic Type and VoiceOver reads the
// text as it is. The tone picks a text colour and the fill that sits behind it;
// the word carries the meaning, the colour only supports it.

import SwiftUI

/// A short label on a tone-coloured capsule.
public struct Pill: View {
    let text: String
    let tone: Tone

    public init(_ text: String, tone: Tone = .neutral) {
        self.text = text
        self.tone = tone
    }

    public var body: some View {
        Text(text)
            .font(Theme.caption.weight(.medium))
            .foregroundStyle(Theme.color(tone))
            .padding(.vertical, Theme.Spacing.xs)
            .padding(.horizontal, Theme.Spacing.sm)
            .background(Capsule().fill(Theme.fill(tone)))
    }
}

extension Pill {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case neutral
        case positive
        case attention
        case negative

        /// The tone for this state.
        public var tone: Tone {
            switch self {
            case .neutral: .neutral
            case .positive: .positive
            case .attention: .attention
            case .negative: .negative
            }
        }

        /// The text for this state.
        public var text: String {
            switch self {
            case .neutral: "Researcher"
            case .positive: "Ready"
            case .attention: "Needs you"
            case .negative: "Stopped"
            }
        }
    }

    /// A pill showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(state.text, tone: state.tone)
    }
}

#Preview("Neutral") { Pill(previewState: .neutral).padding() }
#Preview("Positive") { Pill(previewState: .positive).padding() }
#Preview("Attention") { Pill(previewState: .attention).padding() }
#Preview("Negative") { Pill(previewState: .negative).padding() }
