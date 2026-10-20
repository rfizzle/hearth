// MessageRow is one message in the transcript: an Avatar, the author's name
// and the time over the text, the owner's on the right and on the secondary
// surface. It composes Avatar, TranscriptText while the message is still
// streaming, a Text of the parsed Markdown once it is complete, and a FileRow
// per file the message produced. Inline Markdown only: tables and code blocks
// wait for a Markdown package. A system message is a centred notice with an
// info symbol instead. The name and time read as one element; the Avatar is
// hidden, since the name says who wrote it.

import HearthDesign
import HearthKit
import SwiftUI

/// A message in a conversation.
public struct MessageRow: View {
    let message: Hearth_V1_Message
    let author: Hearth_V1_Bot?

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
        if message.kind == .system {
            notice
        } else {
            HStack(alignment: .top, spacing: 8) {
                if isOwner {
                    Spacer(minLength: 48)
                } else {
                    avatar
                }
                VStack(alignment: isOwner ? .trailing : .leading, spacing: 4) {
                    header
                    content
                    ForEach(message.files, id: \.path) { file in
                        FileRow(file: file)
                    }
                }
                if isOwner {
                    avatar
                } else {
                    Spacer(minLength: 48)
                }
            }
            .accessibilityElement(children: .contain)
        }
    }

    @ViewBuilder
    private var avatar: some View {
        if let author {
            Avatar(bot: author)
        } else if isOwner {
            Avatar.owner
        } else {
            Avatar(.symbol("person.crop.circle.badge.questionmark"))
        }
    }

    private var header: some View {
        HStack(spacing: 6) {
            Text(isOwner ? "You" : author?.name ?? "Someone")
                .font(.headline)
                .foregroundStyle(Theme.color(.textPrimary))
            Text(message.createdAt.date, format: .dateTime.hour().minute())
                .font(Theme.caption)
                .foregroundStyle(Theme.color(.textSecondary))
        }
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private var content: some View {
        if message.streaming {
            TranscriptText(text: message.content)
        } else if isOwner {
            complete
                .padding(8)
                .background(
                    RoundedRectangle(cornerRadius: 8).fill(Theme.color(.surfaceSecondary)))
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

    private var notice: some View {
        HStack(spacing: 6) {
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
        case streaming
        case complete
        case withFile
        case system

        var message: Hearth_V1_Message {
            switch self {
            case .owner:
                return Fixtures.dmMessages[0]
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
#Preview("Streaming") { MessageRow(previewState: .streaming).padding() }
#Preview("Complete") { MessageRow(previewState: .complete).padding() }
#Preview("With a file") { MessageRow(previewState: .withFile).padding() }
#Preview("System") { MessageRow(previewState: .system).padding() }
