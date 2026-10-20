// Avatar is a small disc with initials, an SF Symbol or a single character
// such as an emoji on it. It wraps a Text or an Image on a circle in the
// secondary surface colour. The diameter scales with Dynamic Type. It is
// hidden from VoiceOver: an avatar always sits beside the name it stands for,
// and reading both says the name twice.

import SwiftUI

/// A person's or a bot's mark.
public struct Avatar: View {
    /// What the disc shows.
    public enum Content: Equatable, Sendable {
        /// Up to two initials taken from a name.
        case initials(String)
        /// An SF Symbol, by name.
        case symbol(String)
        /// A character drawn as text, such as an emoji.
        case character(String)
    }

    let content: Content

    @ScaledMetric(relativeTo: .body) private var diameter: CGFloat = 28

    public init(_ content: Content) {
        self.content = content
    }

    public var body: some View {
        mark
            .font(Theme.caption.weight(.semibold))
            .foregroundStyle(Theme.color(.textPrimary))
            .frame(width: diameter, height: diameter)
            .background(Circle().fill(Theme.color(.surfaceSecondary)))
            .accessibilityHidden(true)
    }

    @ViewBuilder
    private var mark: some View {
        switch content {
        case .initials(let name):
            Text(Self.initials(of: name))
        case .symbol(let name):
            Image(systemName: name)
        case .character(let text):
            Text(text)
        }
    }

    /// The first letters of the first two words of `name`, upper-cased.
    static func initials(of name: String) -> String {
        name.split(separator: " ").prefix(2).compactMap(\.first).map(String.init).joined()
            .uppercased()
    }
}

extension Avatar {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case initials
        case symbol
        case character

        /// The content for this state.
        public var content: Content {
            switch self {
            case .initials: .initials("Dana Reyes")
            case .symbol: .symbol("magnifyingglass")
            case .character: .character("🦊")
            }
        }
    }

    /// An avatar showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(state.content)
    }
}

#Preview("Initials") { Avatar(previewState: .initials).padding() }
#Preview("Symbol") { Avatar(previewState: .symbol).padding() }
#Preview("Character") { Avatar(previewState: .character).padding() }
