// ToolCallRow is one thing a bot did with a tool: a quiet line with a symbol
// for the tool and its own plain-English summary, indented under the bot's
// name, and a disclosure to the arguments it was called with and the result it
// got. A small spinner shows while the result is pending and nothing shows
// when it is done. The files the result produced stay visible under it, each a
// FileRow, so Show in Finder never hides behind the disclosure. It is a
// DisclosureRow, so keyboard and VoiceOver behaviour is the system's; whether
// it is open is presentation state.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// A tool call and, once it has one, its result.
public struct ToolCallRow: View {
    let call: Hearth_V1_ToolCall
    let result: Hearth_V1_Message?

    @State private var isExpanded: Bool
    /// Under the bot's name: the avatar's width and the gap after it.
    @ScaledMetric(relativeTo: .body) private var indent: CGFloat =
        Theme.Size.avatar + Theme.Spacing.sm

    /// A row for `call`; `result` is nil while the tool is still running.
    public init(call: Hearth_V1_ToolCall, result: Hearth_V1_Message?, isExpanded: Bool = false) {
        self.call = call
        self.result = result
        _isExpanded = State(initialValue: isExpanded)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            DisclosureRow(isExpanded: $isExpanded) {
                HStack(spacing: Theme.Spacing.sm) {
                    Image(systemName: Self.symbol(for: call.name))
                        .accessibilityHidden(true)
                    Text(Self.summary(call))
                        .lineLimit(1)
                    if result == nil {
                        ProgressView().controlSize(.small)
                    }
                }
            } content: {
                VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                    detail("Arguments", call.argumentsJson)
                    detail("Result", result?.content ?? "Waiting for the result…")
                }
                .padding(.vertical, Theme.Spacing.xs)
            }
            ForEach(result?.files ?? [], id: \.path) { file in
                FileRow(file: file)
                    .padding(.leading, Theme.Spacing.lg)
            }
        }
        .padding(.leading, indent)
        .frame(maxWidth: Theme.Size.readableWidth, alignment: .leading)
    }

    private func detail(_ title: String, _ text: String) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            Text(title)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
            Text(text)
                .font(Theme.monospaced)
                .foregroundStyle(Theme.color(.textPrimary))
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(Theme.Spacing.sm)
                .background(
                    RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                        .fill(Theme.color(.surfaceSecondary)))
        }
        .accessibilityElement(children: .combine)
    }

    /// The symbol for the tool called `name`.
    static func symbol(for name: String) -> String {
        switch name {
        case "web.search": "magnifyingglass"
        case "web.fetch": "globe"
        case "files.write": "doc.badge.plus"
        case "files.read": "doc.text"
        default: "gearshape"
        }
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
        case runningExpanded
        case done
        case expanded
        case wroteAFile

        /// The call message and its result from the DM fixture.
        var messages: (call: Hearth_V1_Message, result: Hearth_V1_Message?) {
            let messages = Fixtures.dmMessages
            switch self {
            case .running, .runningExpanded: return (messages[3], nil)
            case .done, .expanded: return (messages[3], messages[4])
            case .wroteAFile: return (messages[7], messages[8])
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        let messages = state.messages
        self.init(
            call: messages.call.tool, result: messages.result,
            isExpanded: state == .expanded || state == .runningExpanded)
    }
}

#Preview("Running") { ToolCallRow(previewState: .running).padding() }
#Preview("Running, open") { ToolCallRow(previewState: .runningExpanded).padding() }
#Preview("Done") { ToolCallRow(previewState: .done).padding() }
#Preview("Expanded") { ToolCallRow(previewState: .expanded).padding() }
#Preview("Wrote a file") { ToolCallRow(previewState: .wroteAFile).padding() }
