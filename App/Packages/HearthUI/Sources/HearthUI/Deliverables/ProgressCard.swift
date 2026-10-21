// ProgressCard is done out of total on a card: the progress pie the sidebar
// draws for a project, "3 of 7" beside it, the label and the caption. Bound to
// a measure, the count is the live one when the caller has it, else the
// file's with when it was written, else the core's reason in the attention
// tone and an empty pie. A live count in money or bytes is written as such
// ("$45.00 of $100.00"), not as the raw cents or bytes the core counts in.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// Done out of total from a deliverable.
public struct ProgressCard: View {
    let progress: Deliverable.Progress
    let bound: BoundValue

    public init(progress: Deliverable.Progress, bound: BoundValue = .unbound) {
        self.progress = progress
        self.bound = bound
    }

    public var body: some View {
        let count = Self.count(progress, bound)
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            HStack(alignment: .firstTextBaseline) {
                Text(progress.label)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
                Spacer(minLength: Theme.Spacing.sm)
                BoundLabel(bound: bound)
            }
            HStack(spacing: Theme.Spacing.sm) {
                ProgressPie(done: count?.done ?? 0, total: count?.total ?? 0)
                Text(Self.words(count))
                    .font(Theme.title.weight(.semibold))
                    .foregroundStyle(Theme.color(.textPrimary))
                    .monospacedDigit()
            }
            if bound.showsFile {
                CaptionLine(text: progress.caption)
            }
        }
        .deliverableCard()
        .accessibilityElement(children: .combine)
    }

    /// Done and total as shown, with the unit the core counts them in (empty
    /// for the file's): live, written, or nil when there is none.
    nonisolated static func count(_ progress: Deliverable.Progress, _ bound: BoundValue) -> Count? {
        switch bound {
        case .live(let value):
            return Count(done: Int(value.done), total: Int(value.total), unit: value.unit)
        case .missing: return nil
        case .unbound, .written:
            guard let done = progress.done, let total = progress.total else { return nil }
            return Count(done: Int(done), total: Int(total), unit: "")
        }
    }

    /// "3 of 7", "$45.00 of $100.00", "1.2 MB of 2 MB", or a dash with no
    /// count.
    nonisolated static func words(_ count: Count?, locale: Locale = .current) -> String {
        guard let count else { return "—" }
        return "\(count.amount(count.done, locale)) of \(count.amount(count.total, locale))"
    }

    /// Done out of total in the unit the core counts them in.
    struct Count: Equatable {
        let done: Int
        let total: Int
        let unit: String

        func amount(_ value: Int, _ locale: Locale) -> String {
            switch unit {
            case "cents": DeliverableDisplay.number(Double(value) / 100, .currency, locale: locale)
            case "usd_micros":
                DeliverableDisplay.number(Double(value) / 1_000_000, .currency, locale: locale)
            case "bytes": ByteCountFormatter.string(fromByteCount: Int64(value), countStyle: .file)
            default: "\(value)"
            }
        }
    }
}

extension ProgressCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case plain
        case allDone = "all done"
        case written = "bound, as written"
        case live = "bound, live"
        case missing = "bound, missing"

        var widget: Deliverable.Widget { Fixtures.kitchenSink.sections[0].widgets[1] }

        var progress: Deliverable.Progress {
            switch self {
            case .plain:
                return .init(
                    label: "Launch the API tier", done: 3, total: 7, caption: "3 of 7 tasks done")
            case .allDone:
                return .init(
                    label: "Plan the launch event", done: 5, total: 5,
                    caption: "Everything is booked")
            case .written, .live, .missing:
                guard case .progress(let progress) = widget.content else {
                    preconditionFailure("the kitchen sink's second widget is a progress")
                }
                return progress
            }
        }

        var bound: BoundValue {
            switch self {
            case .plain, .allDone: .unbound
            case .written: BoundValue.of(widget, in: nil)
            case .live: BoundValue.of(widget, in: Fixtures.liveMeasures)
            case .missing: BoundValue.of(widget, in: Fixtures.missingMeasures)
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(progress: state.progress, bound: state.bound)
    }
}

#Preview("Live") { ProgressCard(previewState: .live).frame(width: 240).padding() }
