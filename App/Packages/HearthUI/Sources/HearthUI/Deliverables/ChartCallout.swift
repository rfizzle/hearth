// ChartCallout is the small card a chart shows while the pointer is over it:
// the category (or slice) it points at and, per series, the value written in
// the chart's own format, the same formatter the axis uses. The model is plain
// values so a test can read the text the view draws.
// See docs/design.md#colour, docs/decisions.md (D45)

import HearthDesign
import HearthKit
import SwiftUI

/// What a chart says about the place the pointer is on.
struct ChartCallout: Equatable {
    /// One line: a series or slice, its value, and the hue it is drawn in.
    struct Row: Equatable {
        let name: String
        let value: String
        let hue: Int
    }

    let title: String
    let rows: [Row]

    /// The callout for `category` of a line or bar chart: every series' value
    /// there, or nil when the chart has no such category.
    static func category(_ chart: Deliverable.Chart, _ category: String) -> ChartCallout? {
        guard let index = chart.categories.firstIndex(of: category) else { return nil }
        let rows = chart.series.enumerated().compactMap { offset, series -> Row? in
            guard series.values.indices.contains(index) else { return nil }
            return Row(
                name: series.name,
                value: DeliverableDisplay.number(series.values[index], chart.format),
                hue: offset)
        }
        return ChartCallout(title: category, rows: rows)
    }

    /// The callout for the slice at `angle` of a pie, the running total of the
    /// slice values the pointer is at: the label, the value and the share.
    static func slice(_ pie: Deliverable.Pie, at angle: Double) -> ChartCallout? {
        let total = pie.slices.reduce(0) { $0 + $1.value }
        guard total > 0 else { return nil }
        var run = 0.0
        for (offset, slice) in pie.slices.enumerated() {
            run += slice.value
            guard angle <= run, slice.value > 0 else { continue }
            let share = (slice.value / total * 100).rounded(.toNearestOrAwayFromZero)
            let value = DeliverableDisplay.number(slice.value, pie.format)
            return ChartCallout(
                title: slice.label,
                rows: [Row(name: "\(value) (\(Int(share))%)", value: "", hue: offset)])
        }
        return nil
    }

    /// The text a person reads, one part per row.
    var text: String {
        ([title] + rows.map { $0.value.isEmpty ? $0.name : "\($0.name): \($0.value)" })
            .joined(separator: ", ")
    }
}

/// The callout card, in the design's surface and text colours.
struct ChartCalloutView: View {
    let callout: ChartCallout

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(callout.title)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            ForEach(Array(callout.rows.enumerated()), id: \.offset) { _, row in
                HStack(spacing: Theme.Spacing.xs) {
                    Circle().fill(ChartCard.hue(row.hue)).frame(width: 8, height: 8)
                    if row.value.isEmpty {
                        Text(row.name).fontWeight(.semibold)
                    } else {
                        Text(row.name)
                        Text(row.value).fontWeight(.semibold)
                    }
                }
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textPrimary))
            }
        }
        .padding(.horizontal, Theme.Spacing.sm)
        .padding(.vertical, Theme.Spacing.xs + 2)
        .background(
            RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                .fill(Theme.color(.surfaceSecondary))
        )
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                .strokeBorder(Theme.color(.textSecondary).opacity(0.25), lineWidth: 0.5)
        )
        .fixedSize()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
