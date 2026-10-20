// SuggestionChip is a phrase the owner can put in a field: a grey capsule that
// is a plain button, so keyboard and VoiceOver behaviour is the system's. It
// fills, it never sends; the question card's suggested answers and the
// introduction card's example prompts are the same shape, so one capsule
// means one thing. It keeps a minimum target tall around the small pill, and
// VoiceOver reads the phrase as the label and the hint as what a press does.
// See docs/design.md#spacing-and-shape

import HearthDesign
import SwiftUI

/// A phrase that puts itself in a field when pressed.
public struct SuggestionChip: View {
    let text: String
    let hint: String
    let action: () -> Void

    /// A chip for `text`; `hint` says where pressing puts it ("Puts this in
    /// the message field").
    public init(_ text: String, hint: String, action: @escaping () -> Void) {
        self.text = text
        self.hint = hint
        self.action = action
    }

    public var body: some View {
        Button(action: action) {
            Pill(text)
                .frame(minHeight: Theme.Size.minTarget)
                .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .accessibilityLabel(text)
        .accessibilityHint(hint)
    }
}

extension SuggestionChip {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case short
        case longest

        var text: String {
            switch self {
            case .short: "Remember that I prefer short answers"
            case .longest: "What would a heat pump cost for a small house?"
            }
        }
    }

    /// A chip showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(state.text, hint: "Puts this in the message field", action: {})
    }
}

#Preview("Short") { SuggestionChip(previewState: .short).padding() }
#Preview("Longest") { SuggestionChip(previewState: .longest).padding() }
