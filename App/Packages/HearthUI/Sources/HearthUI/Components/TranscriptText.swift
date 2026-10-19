// TranscriptText shows streamed text as it arrives, in the primary text colour
// and the body type token. It wraps a plain Text, so VoiceOver reads it as
// static text and Dynamic Type scales it; no fixed height.

import HearthDesign
import SwiftUI

/// The text of a transcript, as far as it has streamed.
public struct TranscriptText: View {
    let text: String

    public init(text: String) {
        self.text = text
    }

    public var body: some View {
        Text(text)
            .font(Theme.body)
            .foregroundStyle(Theme.color(.textPrimary))
            .frame(maxWidth: .infinity, alignment: .topLeading)
            .textSelection(.enabled)
    }
}

extension TranscriptText {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case empty
        case streaming
        case complete

        /// The fixture text for this state.
        public var text: String {
            switch self {
            case .empty:
                ""
            case .streaming:
                "Hello from the preview"
            case .complete:
                "Hello from the preview client."
            }
        }
    }
}

#Preview("Empty") {
    TranscriptText(text: TranscriptText.PreviewState.empty.text).padding()
}

#Preview("Streaming") {
    TranscriptText(text: TranscriptText.PreviewState.streaming.text).padding()
}

#Preview("Complete") {
    TranscriptText(text: TranscriptText.PreviewState.complete.text).padding()
}
