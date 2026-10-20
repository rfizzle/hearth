// ToolCallRow is one thing a bot did with a tool: a collapsed line with the
// tool's own plain-English summary, and a disclosure to the arguments it was
// called with and the result it got. The files the result produced stay
// visible under it, each a FileRow, so Show in Finder never hides behind the
// disclosure. It wraps a DisclosureGroup, so keyboard and VoiceOver behaviour
// is the system's; whether it is open is presentation state.

import HearthDesign
import HearthKit
import SwiftUI

/// A tool call and, once it has one, its result.
public struct ToolCallRow: View {
    let call: Hearth_V1_ToolCall
    let result: Hearth_V1_Message?

    @State private var isExpanded: Bool

    /// A row for `call`; `result` is nil while the tool is still running.
    public init(call: Hearth_V1_ToolCall, result: Hearth_V1_Message?, isExpanded: Bool = false) {
        self.call = call
        self.result = result
        _isExpanded = State(initialValue: isExpanded)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            DisclosureGroup(isExpanded: $isExpanded) {
                VStack(alignment: .leading, spacing: 4) {
                    detail("Arguments", call.argumentsJson)
                    detail("Result", result?.content ?? "Waiting for the result…")
                }
                .padding(.leading, 4)
            } label: {
                HStack(spacing: 6) {
                    Image(systemName: result == nil ? "gearshape" : "checkmark.circle")
                        .accessibilityHidden(true)
                    Text(Self.summary(call))
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
            }
            ForEach(result?.files ?? [], id: \.path) { file in
                FileRow(file: file)
            }
        }
    }

    private func detail(_ title: String, _ text: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(title)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            Text(text)
                .font(Theme.body.monospaced())
                .foregroundStyle(Theme.color(.textPrimary))
                .textSelection(.enabled)
        }
        .accessibilityElement(children: .combine)
    }

    /// The collapsed line: the tool's summary, else its name.
    static func summary(_ call: Hearth_V1_ToolCall) -> String {
        call.summary.isEmpty ? call.name : call.summary
    }
}

extension ToolCallRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case running
        case done
        case expanded
        case wroteAFile

        /// The call message and its result from the DM fixture.
        var messages: (call: Hearth_V1_Message, result: Hearth_V1_Message?) {
            let messages = Fixtures.dmMessages
            switch self {
            case .running: return (messages[3], nil)
            case .done, .expanded: return (messages[3], messages[4])
            case .wroteAFile: return (messages[7], messages[8])
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        let messages = state.messages
        self.init(
            call: messages.call.tool, result: messages.result, isExpanded: state == .expanded)
    }
}

#Preview("Running") { ToolCallRow(previewState: .running).padding() }
#Preview("Done") { ToolCallRow(previewState: .done).padding() }
#Preview("Expanded") { ToolCallRow(previewState: .expanded).padding() }
#Preview("Wrote a file") { ToolCallRow(previewState: .wroteAFile).padding() }
