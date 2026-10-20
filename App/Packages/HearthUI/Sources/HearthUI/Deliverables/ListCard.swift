// ListCard is a deliverable's list on a card: bullets, numbers, or a check
// list where a done line shows a filled check and an open one an empty
// circle, each said to VoiceOver in words. A line's detail sits under it in
// secondary text.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// A list from a deliverable.
public struct ListCard: View {
    let list: Deliverable.List

    public init(list: Deliverable.List) {
        self.list = list
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            WidgetTitle(text: list.title)
            ForEach(Array(list.items.enumerated()), id: \.offset) { index, item in
                HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
                    marker(index: index, item: item)
                    VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                        Text(item.text)
                            .font(Theme.body)
                            .foregroundStyle(Theme.color(.textPrimary))
                        CaptionLine(text: item.detail)
                    }
                }
                .accessibilityElement(children: .combine)
            }
        }
        .deliverableCard()
    }

    @ViewBuilder private func marker(index: Int, item: Deliverable.Item) -> some View {
        switch list.style {
        case .bullet:
            Text("•").foregroundStyle(Theme.color(.textSecondary)).accessibilityHidden(true)
        case .numbered:
            Text("\(index + 1).")
                .monospacedDigit()
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
        case .check:
            Image(systemName: item.done ? "checkmark.circle.fill" : "circle")
                .foregroundStyle(
                    item.done ? AnyShapeStyle(.tint) : AnyShapeStyle(Theme.color(.textSecondary))
                )
                .accessibilityLabel(item.done ? "Done" : "Not done")
        }
    }
}

extension ListCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case bullet
        case numbered
        case check

        var list: Deliverable.List {
            guard case .list(let check) = Fixtures.kitchenSink.sections[2].widgets[1].content else {
                preconditionFailure("the kitchen sink's second open item is a list")
            }
            let style: Deliverable.ListStyle =
                switch self {
                case .bullet: .bullet
                case .numbered: .numbered
                case .check: .check
                }
            return .init(title: check.title, style: style, items: check.items)
        }
    }

    /// A list showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(list: state.list)
    }
}

#Preview("Check") { ListCard(previewState: .check).frame(width: 360).padding() }
