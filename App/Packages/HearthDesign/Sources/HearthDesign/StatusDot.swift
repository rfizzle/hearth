// StatusDot is a small filled circle coloured by a tone: a bot that is working,
// waiting or in trouble. It wraps an Image of the "circle.fill" SF Symbol, so
// it scales with Dynamic Type through the caption font. When it pulses it uses
// the system's pulse symbol effect, which is switched off under Reduce Motion
// and leaves the plain dot. The colour never stands alone: the label is what
// VoiceOver reads.
// See docs/design.md#colour

import SwiftUI

/// A tone-coloured dot with a spoken label.
public struct StatusDot: View {
    let tone: Tone
    let label: String
    let pulses: Bool

    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    public init(_ tone: Tone, label: String, pulses: Bool = false) {
        self.tone = tone
        self.label = label
        self.pulses = pulses
    }

    public var body: some View {
        Image(systemName: "circle.fill")
            .font(Theme.caption)
            .foregroundStyle(Theme.color(tone))
            .symbolEffect(.pulse, options: .repeating, isActive: pulses && !reduceMotion)
            .accessibilityLabel(label)
    }
}

extension StatusDot {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case positive
        case attention
        case negative
        case pulsing

        /// The tone for this state.
        public var tone: Tone {
            switch self {
            case .positive: .positive
            case .attention, .pulsing: .attention
            case .negative: .negative
            }
        }

        /// Whether the dot pulses in this state.
        public var pulses: Bool { self == .pulsing }

        /// The spoken label for this state.
        public var label: String {
            switch self {
            case .positive: "Ready"
            case .attention: "Needs you"
            case .negative: "Stopped"
            case .pulsing: "Working"
            }
        }
    }

    /// A dot showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(state.tone, label: state.label, pulses: state.pulses)
    }
}

#Preview("Positive") { StatusDot(previewState: .positive).padding() }
#Preview("Attention") { StatusDot(previewState: .attention).padding() }
#Preview("Negative") { StatusDot(previewState: .negative).padding() }
#Preview("Pulsing") { StatusDot(previewState: .pulsing).padding() }
