// TableCard is a deliverable's table on a card: a Grid with the header row in
// the headline style, each column aligned as the spec says, numbers written
// in the column's format with tabular digits, hairlines under the header and
// over the footer. It does not scroll: the window around the deliverable does.
// See docs/design.md#spacing-and-shape

import HearthDesign
import HearthKit
import SwiftUI

/// A table from a deliverable.
public struct TableCard: View {
    let table: Deliverable.Table

    public init(table: Deliverable.Table) {
        self.table = table
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            WidgetTitle(text: table.title)
            Grid(
                alignment: .leading, horizontalSpacing: Theme.Spacing.lg,
                verticalSpacing: Theme.Spacing.xs
            ) {
                GridRow {
                    ForEach(Array(table.columns.enumerated()), id: \.offset) { _, column in
                        Text(column.header)
                            .font(Theme.headline)
                            .foregroundStyle(Theme.color(.textPrimary))
                            .gridColumnAlignment(Self.alignment(column.align))
                    }
                }
                Divider()
                ForEach(Array(table.rows.enumerated()), id: \.offset) { _, row in
                    cells(row, font: Theme.body)
                }
                if let footer = table.footer {
                    Divider()
                    cells(footer, font: Theme.body.weight(.semibold))
                }
            }
        }
        .deliverableCard()
    }

    private func cells(_ row: [Deliverable.Cell], font: Font) -> some View {
        GridRow {
            ForEach(Array(table.columns.enumerated()), id: \.offset) { index, column in
                Text(
                    index < row.count ? DeliverableDisplay.cell(row[index], in: column.format) : ""
                )
                .font(font)
                .monospacedDigit()
                .foregroundStyle(Theme.color(.textPrimary))
            }
        }
    }

    nonisolated static func alignment(_ align: Deliverable.Align) -> HorizontalAlignment {
        switch align {
        case .left: .leading
        case .center: .center
        case .right: .trailing
        }
    }
}

extension TableCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case withFooter = "with a footer"
        case numbers = "numbers, no title"

        var table: Deliverable.Table {
            let widget =
                self == .withFooter
                ? Fixtures.kitchenSink.sections[2].widgets[0]
                : Fixtures.deliverable("one-page-brief").sections[2].widgets[0]
            guard case .table(let table) = widget.content else {
                preconditionFailure("the fixture's widget is a table")
            }
            return table
        }
    }

    /// A table showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(table: state.table)
    }
}

#Preview("With a footer") { TableCard(previewState: .withFooter).frame(width: 480).padding() }
