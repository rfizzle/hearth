// ChartCard draws a line, a bar or a pie chart from a deliverable with Swift
// Charts, on a card under its title. Each series (or slice) takes the chart
// hue of its place in order, the same six the web page uses, wrapping past
// six as the page does, and every hue is named in the legend, so no colour
// stands alone. The framework picks its own ticks: the data is the contract
// with the web page, not its pixels. Values on the axis are written in the
// chart's format for the person reading it; Swift Charts gives VoiceOver the
// audio graph.
// See docs/design.md#colour, docs/decisions.md (D45)

import Charts
import HearthDesign
import HearthKit
import SwiftUI

/// A line, bar or pie chart from a deliverable.
public struct ChartCard: View {
    /// The chart drawn: a line or bar over categories, or a pie.
    public enum Shape: Equatable, Sendable {
        case line(Deliverable.Chart)
        case bar(Deliverable.Chart)
        case pie(Deliverable.Pie)

        /// The chart a widget's content draws, or nil for any other kind.
        public init?(_ content: Deliverable.Content) {
            switch content {
            case .line(let data): self = .line(data)
            case .bar(let data): self = .bar(data)
            case .pie(let pie): self = .pie(pie)
            default: return nil
            }
        }
    }

    let shape: Shape

    public init(_ shape: Shape) {
        self.shape = shape
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            WidgetTitle(text: title)
            chart
                .frame(minHeight: 180)
                .accessibilityLabel(title)
        }
        .deliverableCard()
    }

    private var title: String {
        switch shape {
        case .line(let data), .bar(let data): data.title
        case .pie(let pie): pie.title
        }
    }

    @ViewBuilder private var chart: some View {
        switch shape {
        case .line(let data):
            Chart {
                ForEach(Self.points(data)) { point in
                    LineMark(
                        x: .value("Category", point.category),
                        y: .value(data.yLabel ?? "Value", point.value)
                    )
                    .foregroundStyle(by: .value("Series", point.series))
                    PointMark(
                        x: .value("Category", point.category),
                        y: .value(data.yLabel ?? "Value", point.value)
                    )
                    .foregroundStyle(by: .value("Series", point.series))
                }
            }
            .modifier(Axes(chart: data))
            .chartForegroundStyleScale(
                domain: data.series.map(\.name), range: Self.hues(data.series.count))
        case .bar(let data):
            Chart {
                ForEach(Self.points(data)) { point in
                    if data.stacked {
                        BarMark(
                            x: .value("Category", point.category),
                            y: .value(data.yLabel ?? "Value", point.value)
                        )
                        .foregroundStyle(by: .value("Series", point.series))
                    } else {
                        BarMark(
                            x: .value("Category", point.category),
                            y: .value(data.yLabel ?? "Value", point.value)
                        )
                        .foregroundStyle(by: .value("Series", point.series))
                        .position(by: .value("Series", point.series))
                    }
                }
            }
            .modifier(Axes(chart: data))
            .chartForegroundStyleScale(
                domain: data.series.map(\.name), range: Self.hues(data.series.count))
        case .pie(let pie):
            Chart {
                ForEach(Array(pie.slices.enumerated()), id: \.offset) { _, slice in
                    SectorMark(angle: .value("Value", slice.value), angularInset: 1)
                        .foregroundStyle(by: .value("Slice", slice.label))
                        .accessibilityValue(DeliverableDisplay.number(slice.value, pie.format))
                }
            }
            .chartForegroundStyleScale(
                domain: pie.slices.map(\.label), range: Self.hues(pie.slices.count)
            )
            .chartLegend(position: .trailing, alignment: .center)
        }
    }

    /// The hues for `count` series in order, wrapping past six.
    nonisolated static func hues(_ count: Int) -> [Color] {
        (0..<count).map { Theme.chart($0 % 6 + 1) }
    }

    /// One value of one series at one category.
    struct Point: Identifiable {
        let id: Int
        let series: String
        let category: String
        let value: Double
    }

    /// Every value of `chart` as a point, series by series.
    nonisolated static func points(_ chart: Deliverable.Chart) -> [Point] {
        var out: [Point] = []
        for series in chart.series {
            for (category, value) in zip(chart.categories, series.values) {
                out.append(
                    Point(id: out.count, series: series.name, category: category, value: value))
            }
        }
        return out
    }
}

/// The value axis of a line or bar chart: its bounds when the spec gives them,
/// and its labels in the chart's format.
private struct Axes: ViewModifier {
    let chart: Deliverable.Chart

    func body(content: Content) -> some View {
        content
            .chartYScale(domain: .automatic(includesZero: chart.yMin == nil))
            .modifier(Bounds(low: chart.yMin, high: chart.yMax))
            .chartYAxis {
                AxisMarks { value in
                    AxisGridLine()
                    AxisValueLabel {
                        if let number = value.as(Double.self) {
                            Text(DeliverableDisplay.number(number, chart.format))
                        }
                    }
                }
            }
            .chartYAxisLabel(chart.yLabel ?? "")
    }
}

/// A fixed value axis when the spec gives both ends.
private struct Bounds: ViewModifier {
    let low: Double?
    let high: Double?

    func body(content: Content) -> some View {
        if let low, let high, low < high {
            content.chartYScale(domain: low...high)
        } else {
            content
        }
    }
}

extension ChartCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case line = "line, two series"
        case lineOne = "line, one series"
        case bar = "bar, side by side"
        case stacked = "bar, stacked"
        case pie

        var shape: Shape {
            let trends = Fixtures.kitchenSink.sections[1].widgets.map(\.content)
            switch (self, trends[0], trends[1], trends[2]) {
            case (.line, .line(let data), _, _): return .line(data)
            case (.bar, _, .bar(let data), _): return .bar(data)
            case (.pie, _, _, .pie(let pie)): return .pie(pie)
            case (.lineOne, _, _, _):
                guard
                    case .line(let data) = Fixtures.deliverable("weekly-digest").sections[2]
                        .widgets[0]
                        .content
                else { preconditionFailure("the weekly digest's trend is a line") }
                return .line(data)
            case (.stacked, _, _, _):
                return .bar(
                    .init(
                        title: "Tasks by employee", categories: ["Dana", "Sam", "Ari"],
                        series: [
                            .init(name: "Done", values: [4, 2, 3]),
                            .init(name: "Working", values: [1, 2, 0]),
                            .init(name: "Waiting", values: [1, 0, 2]),
                        ], format: .integer, yLabel: "Tasks", yMin: nil, yMax: nil, stacked: true))
            default:
                preconditionFailure("the kitchen sink's trends are a line, a bar and a pie")
            }
        }
    }

    /// A chart showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(state.shape)
    }
}

#Preview("Line") { ChartCard(previewState: .line).frame(width: 480).padding() }
#Preview("Pie") { ChartCard(previewState: .pie).frame(width: 360).padding() }
