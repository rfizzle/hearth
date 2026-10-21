// A chart's data leaves as tab-separated text a spreadsheet pastes as a table.

import HearthKit
import Testing

@testable import HearthUI

private func chart(_ categories: [String], _ series: [Deliverable.Series]) -> ChartCard.Shape {
    .bar(
        .init(
            title: "T", categories: categories, series: series, format: .integer, yLabel: nil,
            yMin: nil, yMax: nil, stacked: false))
}

@Test func oneSeriesIsAHeaderAndARowPerCategory() {
    let shape = chart(["Mon", "Tue"], [.init(name: "Tasks", values: [3, 4.5])])
    #expect(ChartExport.tabSeparated(shape) == "Category\tTasks\nMon\t3\nTue\t4.5\n")
}

@Test func severalSeriesAreOneColumnEach() {
    let shape = chart(
        ["Mon", "Tue"], [.init(name: "Done", values: [1, 2]), .init(name: "Open", values: [5])])
    #expect(ChartExport.tabSeparated(shape) == "Category\tDone\tOpen\nMon\t1\t5\nTue\t2\t\n")
}

@Test func aPieIsASliceAndAValueColumn() {
    guard case .pie(let pie) = ChartCard.PreviewState.pie.shape else {
        Issue.record("the pie state is a pie")
        return
    }
    let rows = ChartExport.tabSeparated(.pie(pie)).split(separator: "\n").map(String.init)
    #expect(rows.first == "Slice\tValue")
    #expect(rows.count == pie.slices.count + 1)
    #expect(rows[1] == "\(pie.slices[0].label)\t\(ChartExport.plain(pie.slices[0].value))")
}

@Test func tabsAndNewlinesInLabelsBecomeSpaces() {
    let shape = chart(["a\tb\nc\r\nd"], [.init(name: "x\ty", values: [1])])
    #expect(ChartExport.tabSeparated(shape) == "Category\tx y\na b c d\t1\n")
}
