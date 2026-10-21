// The callout a chart shows under the pointer says the value the way the axis
// does, and a paragraph with a link is the one that shows the hand.

import HearthKit
import Testing

@testable import HearthUI

@Test func aLineCalloutReadsEverySeriesInTheChartsFormat() throws {
    guard case .line(let data) = ChartCard.PreviewState.line.shape else {
        Issue.record("the line state is a line")
        return
    }
    let callout = try #require(ChartCallout.category(data, data.categories[1]))
    #expect(callout.title == data.categories[1])
    #expect(callout.rows.map(\.name) == data.series.map(\.name))
    #expect(
        callout.rows.map(\.value)
            == data.series.map { DeliverableDisplay.number($0.values[1], data.format) })
    #expect(ChartCallout.category(data, "no such category") == nil)
}

@Test func aPieCalloutReadsTheSliceUnderTheAngleWithItsShare() throws {
    guard case .pie(let pie) = ChartCard.PreviewState.pie.shape else {
        Issue.record("the pie state is a pie")
        return
    }
    let total = pie.slices.reduce(0) { $0 + $1.value }
    let first = try #require(ChartCallout.slice(pie, at: pie.slices[0].value / 2))
    #expect(first.title == pie.slices[0].label)
    let share = Int((pie.slices[0].value / total * 100).rounded())
    let value = DeliverableDisplay.number(pie.slices[0].value, pie.format)
    #expect(first.rows.first?.name == "\(value) (\(share)%)")
    let last = try #require(ChartCallout.slice(pie, at: total))
    #expect(last.title == pie.slices.last?.label)
    #expect(ChartCallout.slice(pie, at: total + 1) == nil)
}

@Test func onlyAParagraphWithALinkShowsTheHand() {
    #expect(DeliverableMarkdown.hasLink("See [the report](https://example.com/r) now."))
    #expect(DeliverableMarkdown.hasLink("Mail <mailto:a@example.com>."))
    #expect(!DeliverableMarkdown.hasLink("Plain words, no link."))
}
