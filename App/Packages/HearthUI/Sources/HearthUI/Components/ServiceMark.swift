// ServiceMark is the picture on a provider's or an integration's card: the
// first letter of its name in the large avatar disc. It is a monogram and not
// the company's logo because OpenAI, Anthropic, Google, OpenRouter and Brave
// each publish brand guidelines, hearth is MIT-licensed, and shipping their
// marks needs each company's permission, where the disc is hearth's own. It is
// a letter and not an SF Symbol because every service here is the same kind of
// thing, so a symbol apiece would invent a difference where a letter states
// the honest one. It wraps Avatar, which is hidden from VoiceOver: the card
// beside it says the name.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// A one-letter mark for a service.
public struct ServiceMark: View {
    let letter: String

    /// A mark showing the first letter of `name`.
    public init(named name: String) {
        letter = String(name.prefix(1)).uppercased()
    }

    /// The mark for `provider`.
    public init(_ provider: Hearth_V1_Provider) {
        self.init(named: provider.displayName)
    }

    public var body: some View {
        Avatar(.initials(letter), size: .large)
    }
}

extension ServiceMark {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case chatgpt
        case anthropic
        case gemini
        case openrouter
        case brave

        var name: String {
            switch self {
            case .chatgpt: Hearth_V1_Provider.chatgpt.displayName
            case .anthropic: Hearth_V1_Provider.anthropic.displayName
            case .gemini: Hearth_V1_Provider.gemini.displayName
            case .openrouter: Hearth_V1_Provider.openrouter.displayName
            case .brave: "Brave Search"
            }
        }
    }

    /// A mark showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(named: state.name)
    }
}

#Preview("ChatGPT") { ServiceMark(previewState: .chatgpt).padding() }
#Preview("Brave") { ServiceMark(previewState: .brave).padding() }
