import Foundation
import FoundationModels

/// Gives a conversation a short title with the on-device model.
public struct OnDeviceTitler: Sendable {
    /// The session holds 4,096 tokens, instructions and answer included. The
    /// tokenizer is not exposed, so the input is cut by characters instead:
    /// 4,000 characters is at most about 2,000 tokens even at two characters
    /// per token, which leaves room for the instructions and the reply. A title
    /// needs the opening of a text, not the end of it.
    static let maxInputCharacters = 4_000
    static let maxWords = 6

    private let availability: OnDeviceAvailability

    public init(availability: OnDeviceAvailability = .system) {
        self.availability = availability
    }

    /// A title of at most six words, or nil when the model is unavailable, the
    /// text is empty, the call fails, or the model ignores the limit.
    public func title(for text: String) async -> String? {
        guard availability.isAvailable() else { return nil }
        let input = Self.truncated(text)
        guard !input.isEmpty else { return nil }
        let session = LanguageModelSession(
            instructions:
                "Write a title of at most six words for the text. Answer with the title only."
        )
        guard let response = try? await session.respond(to: input) else { return nil }
        return Self.clean(response.content)
    }

    static func truncated(_ text: String) -> String {
        String(text.trimmingCharacters(in: .whitespacesAndNewlines).prefix(maxInputCharacters))
    }

    /// Strips quotes and a trailing full stop; nil when empty or over the limit.
    static func clean(_ raw: String) -> String? {
        var title = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        title = title.trimmingCharacters(in: CharacterSet(charactersIn: "\"'“”‘’"))
        if title.hasSuffix(".") { title.removeLast() }
        let words = title.split(whereSeparator: \.isWhitespace)
        guard !words.isEmpty, words.count <= maxWords else { return nil }
        return words.joined(separator: " ")
    }
}
