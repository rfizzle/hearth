// DeliverableView draws a shaped deliverable natively from its decoded spec:
// the title and subtitle, then each section as a heading over a Grid of
// cards, rows filled left to right up to the section's columns and a widget
// spanning as many columns as it asks. Bound widgets take their figure from
// the live values the caller read, keyed by measure id, or show the file's
// with when it was written. A file this Hearth cannot draw is one placeholder
// card under its title. It does not scroll; the window around it does. The app
// never loads the deliverable's web page: this is what it shows instead.
// See docs/model.md#work, docs/decisions.md (D45)

import HearthDesign
import HearthKit
import SwiftUI

/// A shaped deliverable as cards in a grid.
public struct DeliverableView: View {
    let spec: Deliverable
    let values: [String: Hearth_V1_MeasureValue]?

    /// The deliverable `spec`, with `values` the live reads of its measures
    /// by id, or nil when none were read.
    public init(spec: Deliverable, values: [String: Hearth_V1_MeasureValue]? = nil) {
        self.spec = spec
        self.values = values
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xl) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(spec.title.isEmpty ? "Untitled" : spec.title)
                    .font(Theme.title.weight(.semibold))
                    .foregroundStyle(Theme.color(.textPrimary))
                    .accessibilityAddTraits(.isHeader)
                if let subtitle = spec.subtitle, !subtitle.isEmpty {
                    Text(subtitle)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
            }
            if spec.unsupported != nil {
                UnsupportedCard(detail: UnsupportedCard.file)
            } else {
                ForEach(Array(spec.sections.enumerated()), id: \.offset) { _, section in
                    sectionView(section)
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func sectionView(_ section: Deliverable.Section) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            if let heading = section.heading, !heading.isEmpty {
                Text(heading)
                    .font(Theme.headline)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .accessibilityAddTraits(.isHeader)
            }
            Grid(horizontalSpacing: Theme.Spacing.md, verticalSpacing: Theme.Spacing.md) {
                ForEach(Array(Self.rows(section).enumerated()), id: \.offset) { _, row in
                    GridRow {
                        ForEach(Array(row.enumerated()), id: \.offset) { _, cell in
                            widgetView(cell.widget)
                                .gridCellColumns(cell.span)
                        }
                        ForEach(0..<Self.free(row, columns: section.columns), id: \.self) { _ in
                            Color.clear
                                .frame(maxWidth: .infinity, maxHeight: 0)
                                .gridCellUnsizedAxes(.vertical)
                        }
                    }
                }
            }
        }
    }

    @ViewBuilder private func widgetView(_ widget: Deliverable.Widget) -> some View {
        let bound = BoundValue.of(widget, in: values)
        if let shape = ChartCard.Shape(widget.content) {
            ChartCard(shape)
        } else {
            card(widget.content, bound: bound)
        }
    }

    @ViewBuilder private func card(_ content: Deliverable.Content, bound: BoundValue) -> some View {
        switch content {
        case .metric(let metric): MetricTile(metric: metric, bound: bound)
        case .line, .bar, .pie: EmptyView()  // drawn by ChartCard above
        case .table(let table): TableCard(table: table)
        case .list(let list): ListCard(list: list)
        case .progress(let progress): ProgressCard(progress: progress, bound: bound)
        case .light(let light): LightCard(light: light, bound: bound)
        case .callout(let callout): CalloutCard(callout: callout)
        case .text(let markdown): TextBlock(markdown: markdown)
        case .unsupported(let kind): UnsupportedCard(detail: UnsupportedCard.widget(kind))
        }
    }

    /// One widget in a row with the columns it takes.
    struct Cell {
        let widget: Deliverable.Widget
        let span: Int
    }

    /// The section's widgets in rows: each row takes widgets in order until
    /// the next would not fit in its columns; a span is held to 1 through the
    /// columns.
    nonisolated static func rows(_ section: Deliverable.Section) -> [[Cell]] {
        let columns = max(section.columns, 1)
        var rows: [[Cell]] = []
        var row: [Cell] = []
        var used = 0
        for widget in section.widgets {
            let span = min(max(widget.span, 1), columns)
            if used + span > columns {
                rows.append(row)
                row = []
                used = 0
            }
            row.append(Cell(widget: widget, span: span))
            used += span
        }
        if !row.isEmpty { rows.append(row) }
        return rows
    }

    /// The columns a row leaves empty.
    nonisolated static func free(_ row: [Cell], columns: Int) -> Int {
        max(max(columns, 1) - row.reduce(0) { $0 + $1.span }, 0)
    }
}

extension DeliverableView {
    /// The states the gallery and previews show: every widget as written,
    /// live and missing, a file from a newer Hearth, and each shipped
    /// template's sample.
    public enum PreviewState: String, CaseIterable, Sendable {
        case kitchenSink = "every widget, as written"
        case live = "every widget, live"
        case missing = "every widget, measures missing"
        case newer = "made by a newer Hearth"
        case comparison
        case dashboard
        case onePageBrief = "one-page-brief"
        case report
        case weeklyDigest = "weekly-digest"

        var spec: Deliverable {
            switch self {
            case .kitchenSink, .live, .missing: Fixtures.kitchenSink
            case .newer: Fixtures.newerDeliverable
            case .comparison, .dashboard, .onePageBrief, .report, .weeklyDigest:
                Fixtures.deliverable(rawValue)
            }
        }

        var values: [String: Hearth_V1_MeasureValue]? {
            switch self {
            case .live: Fixtures.liveMeasures
            case .missing: Fixtures.missingMeasures
            default: nil
            }
        }
    }

    /// A deliverable showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(spec: state.spec, values: state.values)
    }
}

#Preview("Every widget") {
    ScrollView { DeliverableView(previewState: .kitchenSink).padding() }.frame(
        width: 720, height: 900)
}
#Preview("Report") {
    ScrollView { DeliverableView(previewState: .report).padding() }.frame(width: 640, height: 800)
}
