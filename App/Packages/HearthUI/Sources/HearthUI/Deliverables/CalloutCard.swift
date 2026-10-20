// CalloutCard is a note in a tone: the card on the tone's quiet fill, a
// symbol whose shape differs by tone, the title in the tone's colour, and the
// Markdown under it. The title is the word the tone travels with; without
// one, the tone's own name stands in its place, so the colour never stands
// alone.
// See docs/design.md#colour

import HearthDesign
import HearthKit
import SwiftUI

/// A note in a tone from a deliverable.
public struct CalloutCard: View {
    let callout: Deliverable.Callout

    public init(callout: Deliverable.Callout) {
        self.callout = callout
    }

    public var body: some View {
        let tone = DeliverableDisplay.tone(callout.tone)
        let shape = RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
                Image(systemName: Self.symbol(tone))
                    .foregroundStyle(Theme.color(tone))
                    .accessibilityHidden(true)
                Text(callout.title.flatMap { $0.isEmpty ? nil : $0 } ?? Self.word(tone))
                    .font(Theme.headline)
                    .foregroundStyle(Theme.color(tone))
                    .accessibilityAddTraits(.isHeader)
            }
            DeliverableMarkdown(markdown: callout.markdown)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .padding(Theme.Spacing.md)
        .background(shape.fill(Theme.fill(tone)))
        .overlay(shape.strokeBorder(.separator, lineWidth: 1))
        .accessibilityElement(children: .combine)
    }

    nonisolated static func symbol(_ tone: Tone) -> String {
        switch tone {
        case .neutral: "info.circle.fill"
        case .positive: "checkmark.circle.fill"
        case .attention: "exclamationmark.circle.fill"
        case .negative: "xmark.octagon.fill"
        }
    }

    /// The tone's name, shown when the callout has no title.
    nonisolated static func word(_ tone: Tone) -> String {
        switch tone {
        case .neutral: "Note"
        case .positive: "Good news"
        case .attention: "Needs you"
        case .negative: "Problem"
        }
    }
}

extension CalloutCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case neutral
        case positive
        case attention
        case negative
        case untitled = "no title"

        var callout: Deliverable.Callout {
            switch self {
            case .neutral:
                .init(
                    tone: .neutral, title: "Note", markdown: "The figures are for October so far.")
            case .positive:
                .init(
                    tone: .positive, title: "Recommended",
                    markdown: "Book the **second** venue; it is $15 more and a day sooner.")
            case .attention:
                .init(
                    tone: .attention, title: "Needs you",
                    markdown:
                        "Two tasks are waiting on an answer. **Open the Inbox** to unblock them.")
            case .negative:
                .init(
                    tone: .negative, title: "Stopped",
                    markdown: "The venue booking failed twice; the site refused the card.")
            case .untitled:
                .init(
                    tone: .attention, title: nil,
                    markdown: "Nobody has answered the venue's email yet.")
            }
        }
    }

    /// A callout showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(callout: state.callout)
    }
}

#Preview("Attention") { CalloutCard(previewState: .attention).frame(width: 360).padding() }
