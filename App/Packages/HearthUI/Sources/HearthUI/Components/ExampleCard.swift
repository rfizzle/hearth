// ExampleCard is one example an empty list offers: a symbol, a title and one
// line on a card, a plain button that starts a setup with the example filled
// in, never one that creates anything. The empty team shows the role templates
// as these and the empty project list a fixed few, so one card means "start
// from this". The card is the same surface as the other cards and takes the
// accent hairline under the pointer, with Theme.quick and nothing under Reduce
// Motion. VoiceOver reads the title with the line as its hint.
// See docs/design.md#spacing-and-shape and docs/design.md#motion

import HearthDesign
import SwiftUI

/// A symbol, a title and one line that start a setup when pressed.
public struct ExampleCard: View {
    let symbol: String
    let title: String
    let line: String
    let action: () -> Void

    @State private var isHovering = false
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `title` and `line` under `symbol`; `action` starts the setup.
    public init(symbol: String, title: String, line: String, action: @escaping () -> Void) {
        self.symbol = symbol
        self.title = title
        self.line = line
        self.action = action
    }

    public var body: some View {
        Button(action: action) {
            HStack(alignment: .top, spacing: Theme.Spacing.md) {
                Image(systemName: symbol)
                    .font(Theme.headline)
                    .foregroundStyle(.tint)
                    .frame(width: Theme.Spacing.xl)
                    .accessibilityHidden(true)
                VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                    Text(title)
                        .font(Theme.headline)
                        .foregroundStyle(Theme.color(.textPrimary))
                    Text(line)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                        .lineLimit(nil)
                        .fixedSize(horizontal: false, vertical: true)
                }
                Spacer(minLength: 0)
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .cardSurface(isSelected: isHovering)
            .contentShape(RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous))
        }
        .buttonStyle(.plain)
        .onHover { isHovering = $0 }
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: isHovering)
        .accessibilityLabel(title)
        .accessibilityHint(line)
    }
}

extension ExampleCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case short
        case longest

        var title: String {
            switch self {
            case .short: "Plan a trip"
            case .longest: "Research Lead for the whole household"
            }
        }

        var line: String {
            switch self {
            case .short: "Where to go, what to book and when."
            case .longest:
                "Finds sources, reads them and writes short briefs you can read in a minute."
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(symbol: "map", title: state.title, line: state.line, action: {})
    }
}

#Preview("Short") { ExampleCard(previewState: .short).padding().frame(width: 280) }
#Preview("Longest") { ExampleCard(previewState: .longest).padding().frame(width: 280) }
