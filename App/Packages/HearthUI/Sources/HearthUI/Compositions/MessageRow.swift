// MessageRow is one message in the transcript. A bot's message is flat in the
// readable column: an Avatar, the name and the time over the text, and a
// FileRow per file it produced, the way a thread reads in a mail client. The
// owner's is a quiet grey bubble on the trailing edge with no name, and its
// time is help text and the accessibility value. A system message is a
// centred notice. The text is TranscriptText while the message is still
// streaming and parsed inline Markdown once it is complete; tables and code
// blocks wait for a Markdown package. There is no streaming cursor: the status
// line is the one working signal. The name and time read as one element; the
// Avatar is hidden, since the name says who wrote it. An approval request or a
// question the owner has answered is a quiet line under the bot's turn saying
// what it asked (an open one is its card, which the screen draws instead), and
// the owner's decision is a notice like a system message.
// See docs/design.md#the-transcript and docs/model.md#8-approvals

import HearthDesign
import HearthKit
import SwiftUI

/// A message in a conversation.
public struct MessageRow: View {
    let message: Hearth_V1_Message
    let author: Hearth_V1_Bot?

    /// Under the bot's name: the avatar's width and the gap after it.
    @ScaledMetric(relativeTo: .body) private var indent: CGFloat =
        Theme.Size.avatar + Theme.Spacing.sm

    /// A row for `message`; `author` is the bot that wrote it, nil for the
    /// owner.
    public init(message: Hearth_V1_Message, author: Hearth_V1_Bot?) {
        self.message = message
        self.author = author
    }

    var isOwner: Bool {
        message.authorBotID.isEmpty
    }

    public var body: some View {
        if message.kind == .system || message.kind == .approvalDecision {
            notice
        } else if message.kind == .approvalRequest || message.kind == .question {
            askedLine
        } else if isOwner {
            ownerBubble
        } else {
            botMessage
        }
    }

    private var botMessage: some View {
        HStack(alignment: .top, spacing: Theme.Spacing.sm) {
            avatar
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                header
                content
                ForEach(message.files, id: \.path) { file in
                    FileRow(file: file)
                }
            }
        }
        .frame(maxWidth: Theme.Size.readableWidth, alignment: .leading)
        .accessibilityElement(children: .contain)
    }

    private var ownerBubble: some View {
        VStack(alignment: .trailing, spacing: Theme.Spacing.xs) {
            complete
                .padding(.horizontal, Theme.Spacing.md)
                .padding(.vertical, Theme.Spacing.sm)
                .background(
                    RoundedRectangle(cornerRadius: Theme.Radius.lg, style: .continuous)
                        .fill(Theme.color(.surfaceSecondary))
                )
                .frame(maxWidth: Theme.Size.bubbleWidth, alignment: .trailing)
                .help(timeText)
                .accessibilityValue(timeText)
            ForEach(message.files, id: \.path) { file in
                FileRow(file: file)
            }
        }
        .frame(maxWidth: Theme.Size.readableWidth, alignment: .trailing)
        .accessibilityElement(children: .contain)
    }

    @ViewBuilder
    private var avatar: some View {
        if let author {
            Avatar(bot: author)
        } else {
            Avatar(.symbol("person.crop.circle.badge.questionmark"))
        }
    }

    private var timeText: String {
        message.createdAt.date.formatted(.dateTime.hour().minute())
    }

    private var header: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
            Text(author?.name ?? "Someone")
                .font(Theme.headline)
                .foregroundStyle(Theme.color(.textPrimary))
            Spacer(minLength: Theme.Spacing.sm)
            Text(timeText)
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
        }
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private var content: some View {
        if message.streaming {
            TranscriptText(text: message.content)
        } else {
            complete
        }
    }

    private var complete: some View {
        Text(Self.markdown(message.content))
            .font(Theme.body)
            .foregroundStyle(Theme.color(.textPrimary))
            .textSelection(.enabled)
    }

    private var askedLine: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Image(systemName: message.kind == .question ? "questionmark.bubble" : "hand.raised")
                .accessibilityHidden(true)
            Text(message.content)
                .lineLimit(2)
        }
        .font(Theme.callout)
        .foregroundStyle(Theme.color(.textSecondary))
        .padding(.leading, indent)
        .frame(maxWidth: Theme.Size.readableWidth, alignment: .leading)
        .accessibilityElement(children: .combine)
        .accessibilityLabel("Asked: \(message.content)")
    }

    private var notice: some View {
        HStack(spacing: Theme.Spacing.xs) {
            Image(systemName: "info.circle").accessibilityHidden(true)
            Text(message.content)
        }
        .font(Theme.caption)
        .foregroundStyle(Theme.color(.textSecondary))
        .frame(maxWidth: .infinity)
        .accessibilityElement(children: .combine)
    }

    /// `text` with its inline Markdown applied, or as typed when it does not
    /// parse.
    static func markdown(_ text: String) -> AttributedString {
        let options = AttributedString.MarkdownParsingOptions(
            interpretedSyntax: .inlineOnlyPreservingWhitespace)
        return (try? AttributedString(markdown: text, options: options)) ?? AttributedString(text)
    }
}

extension MessageRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case owner
        case ownerLong
        case streaming
        case complete
        case withFile
        case system
        case askedApproval
        case askedQuestion
        case decision

        var message: Hearth_V1_Message {
            switch self {
            case .owner:
                return Fixtures.dmMessages[0]
            case .ownerLong:
                var message = Fixtures.dmMessages[0]
                message.content =
                    message.content
                    + " I want it to cover what it costs to install, what it costs to run, "
                    + "what grants there are, and whether the walls and loft need doing first."
                return message
            case .streaming:
                return Fixtures.streamingReply
            case .complete:
                var message = Fixtures.streamingReply
                message.content =
                    "I've written **the brief**. " + Fixtures.replyDeltas[1...].joined()
                message.streaming = false
                return message
            case .withFile:
                var message = Fixtures.streamingReply
                message.content = "Here it is."
                message.streaming = false
                message.files = [Fixtures.briefFile]
                return message
            case .system:
                return Fixtures.dmMessages[1]
            case .askedApproval:
                return Fixtures.approvalRequest
            case .askedQuestion:
                var message = Fixtures.approvalRequest
                message.id = Fixtures.question.messageID
                message.kind = .question
                message.tool = Hearth_V1_ToolCall()
                message.content = Fixtures.question.question
                return message
            case .decision:
                return Fixtures.approvalDecision
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        let message = state.message
        self.init(message: message, author: message.authorBotID.isEmpty ? nil : Fixtures.bot)
    }
}

#Preview("Owner") { MessageRow(previewState: .owner).padding() }
#Preview("Long owner") { MessageRow(previewState: .ownerLong).padding() }
#Preview("Streaming") { MessageRow(previewState: .streaming).padding() }
#Preview("Complete") { MessageRow(previewState: .complete).padding() }
#Preview("With a file") { MessageRow(previewState: .withFile).padding() }
#Preview("System") { MessageRow(previewState: .system).padding() }
#Preview("Asked for approval") { MessageRow(previewState: .askedApproval).padding() }
#Preview("Asked a question") { MessageRow(previewState: .askedQuestion).padding() }
#Preview("Decision") { MessageRow(previewState: .decision).padding() }
