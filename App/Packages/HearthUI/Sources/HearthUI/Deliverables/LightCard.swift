// LightCard is a status on a card: the label, a dot in the tone and the word
// that says the same thing, so the colour never stands alone. Bound to a
// measure, the light is the live one when the caller has it, else the file's
// with when it was written, else the core's reason in the attention tone.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// A status light from a deliverable.
public struct LightCard: View {
    let light: Deliverable.Light
    let bound: BoundValue

    public init(light: Deliverable.Light, bound: BoundValue = .unbound) {
        self.light = light
        self.bound = bound
    }

    public var body: some View {
        let shown = Self.shown(light, bound)
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            HStack(alignment: .firstTextBaseline) {
                Text(light.label)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
                Spacer(minLength: Theme.Spacing.sm)
                BoundLabel(bound: bound)
            }
            HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
                if let word = shown.word {
                    StatusDot(shown.tone, label: word)
                    Text(word)
                        .font(Theme.title.weight(.semibold))
                        .foregroundStyle(Theme.color(.textPrimary))
                } else {
                    Text("—")
                        .font(Theme.title.weight(.semibold))
                        .foregroundStyle(Theme.color(.textSecondary))
                }
            }
            CaptionLine(text: light.caption)
        }
        .deliverableCard()
        .accessibilityElement(children: .combine)
    }

    /// The tone and word shown: live, written, or no word when there is none.
    nonisolated static func shown(_ light: Deliverable.Light, _ bound: BoundValue) -> (
        tone: Tone, word: String?
    ) {
        switch bound {
        case .live(let value):
            return (DeliverableDisplay.tone(value.tone), value.word.isEmpty ? nil : value.word)
        case .missing:
            return (.neutral, nil)
        case .unbound, .written:
            return (DeliverableDisplay.tone(light.tone), light.word)
        }
    }
}

extension LightCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case positive
        case negative
        case written = "bound, as written"
        case live = "bound, live"
        case missing = "bound, missing"

        var widget: Deliverable.Widget { Fixtures.kitchenSink.sections[0].widgets[2] }

        var light: Deliverable.Light {
            switch self {
            case .positive:
                return .init(label: "Newsletter", tone: .positive, word: "On track")
            case .negative:
                return .init(
                    label: "Venue", tone: .negative, word: "Stopped",
                    caption: "The booking failed twice")
            case .written, .live, .missing:
                guard case .light(let light) = widget.content else {
                    preconditionFailure("the kitchen sink's third widget is a light")
                }
                return light
            }
        }

        var bound: BoundValue {
            switch self {
            case .positive, .negative: .unbound
            case .written: BoundValue.of(widget, in: nil)
            case .live: BoundValue.of(widget, in: Fixtures.liveMeasures)
            case .missing: BoundValue.of(widget, in: Fixtures.missingMeasures)
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(light: state.light, bound: state.bound)
    }
}

#Preview("Live") { LightCard(previewState: .live).frame(width: 240).padding() }
