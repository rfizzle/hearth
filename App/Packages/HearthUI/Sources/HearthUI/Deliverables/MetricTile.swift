// MetricTile is one figure on a card: the label, the value large, the change
// beside it as a pill in a tone (good reads positive, bad negative, the signed
// number is the word), and the caption. Bound to a measure, the value is the
// live one when the caller has it, else the file's with when it was written,
// else a dash and the core's reason.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// One figure from a deliverable.
public struct MetricTile: View {
    let metric: Deliverable.Metric
    let bound: BoundValue

    public init(metric: Deliverable.Metric, bound: BoundValue = .unbound) {
        self.metric = metric
        self.bound = bound
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            HStack(alignment: .firstTextBaseline) {
                Text(metric.label)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
                Spacer(minLength: Theme.Spacing.sm)
                BoundLabel(bound: bound)
            }
            HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
                Text(Self.value(metric, bound))
                    .font(Theme.title.weight(.semibold))
                    .foregroundStyle(Theme.color(.textPrimary))
                    .monospacedDigit()
                if let delta = metric.delta, bound.showsFile {
                    Pill(Self.delta(delta, metric), tone: Self.tone(delta))
                        .accessibilityLabel(spoken(delta))
                }
            }
            CaptionLine(text: metric.caption)
        }
        .deliverableCard()
        .accessibilityElement(children: .combine)
    }

    /// The figure as shown: live, written, or a dash when there is none.
    nonisolated static func value(
        _ metric: Deliverable.Metric, _ bound: BoundValue, locale: Locale = .current
    ) -> String {
        let number: Double?
        switch bound {
        case .live(let value): number = DeliverableDisplay.reading(value)
        case .missing: number = nil
        case .unbound, .written: number = metric.value
        }
        guard let number else { return "—" }
        let text = DeliverableDisplay.number(
            number, metric.format, decimals: metric.decimals, locale: locale)
        guard let unit = metric.unit, !unit.isEmpty else { return text }
        return "\(text) \(unit)"
    }

    nonisolated static func delta(
        _ delta: Deliverable.Delta, _ metric: Deliverable.Metric, locale: Locale = .current
    ) -> String {
        let size = DeliverableDisplay.number(
            abs(delta.value), metric.format, decimals: metric.decimals, locale: locale)
        return (delta.value < 0 ? "−" : "+") + size
    }

    nonisolated static func tone(_ delta: Deliverable.Delta) -> Tone {
        switch delta.good {
        case true: .positive
        case false: .negative
        case nil: .neutral
        }
    }

    private func spoken(_ delta: Deliverable.Delta) -> String {
        let way = delta.value < 0 ? "Down" : "Up"
        let size = DeliverableDisplay.number(
            abs(delta.value), metric.format, decimals: metric.decimals)
        let verdict = delta.good.map { $0 ? ", good" : ", not good" } ?? ""
        return "\(way) \(size)\(verdict)"
    }
}

extension MetricTile {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case plain
        case badChange = "bad change"
        case written = "bound, as written"
        case live = "bound, live"
        case missing = "bound, missing"

        var widget: Deliverable.Widget { Fixtures.kitchenSink.sections[0].widgets[0] }

        var metric: Deliverable.Metric {
            switch self {
            case .plain:
                guard
                    case .metric(let metric) = Fixtures.deliverable("dashboard").sections[0]
                        .widgets[2]
                        .content
                else { preconditionFailure("the dashboard's third status widget is a metric") }
                return metric
            case .badChange:
                return .init(
                    label: "Tokens this week", value: 182_400, format: .integer, decimals: nil,
                    unit: "tokens", delta: .init(value: 40_100, good: false),
                    caption: "Up on last week")
            case .written, .live, .missing:
                guard case .metric(let metric) = widget.content else {
                    preconditionFailure("the kitchen sink opens with a metric")
                }
                return metric
            }
        }

        var bound: BoundValue {
            switch self {
            case .plain, .badChange: .unbound
            case .written: BoundValue.of(widget, in: nil)
            case .live: BoundValue.of(widget, in: Fixtures.liveMeasures)
            case .missing: BoundValue.of(widget, in: Fixtures.missingMeasures)
            }
        }
    }

    /// A tile showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(metric: state.metric, bound: state.bound)
    }
}

#Preview("Plain") { MetricTile(previewState: .plain).frame(width: 240).padding() }
#Preview("Live") { MetricTile(previewState: .live).frame(width: 240).padding() }
#Preview("Missing") { MetricTile(previewState: .missing).frame(width: 240).padding() }
