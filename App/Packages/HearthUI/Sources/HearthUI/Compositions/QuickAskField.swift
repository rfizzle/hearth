// QuickAskField is the one-line ask the menu bar extra and the quick-ask
// window share, Alfred's single field: a menu of who to ask (their Avatar and
// name), a plain field on a quaternary fill, and a round Send. Return sends.
// Once something is asked, the conversation's newest two messages follow as
// MessageRows, six lines at most each, then "Open in Hearth" to carry on in
// the window. The reply area is the person's send, so it moves in; the text
// streaming into it does not. The field takes focus when it appears.
// See docs/design.md#window-anatomy and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// A one-line ask with its reply.
public struct QuickAskField: View {
    let bots: [Hearth_V1_Bot]
    let bot: Hearth_V1_Bot?
    let draft: Binding<String>
    let reply: [Hearth_V1_Message]
    let replyAuthor: Hearth_V1_Bot?
    let canSend: Bool
    let error: String?
    let onPick: (Hearth_V1_Bot) -> Void
    let onSend: () -> Void
    let onOpen: () -> Void

    @FocusState private var isFocused: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A field asking `bot`, one of `bots`, with `draft`; `reply` is what the
    /// conversation shows since, newest last, from `replyAuthor` (`bot` when
    /// nil). `onPick` changes who is asked,
    /// `onSend` asks, and `onOpen` opens the conversation in the window.
    public init(
        bots: [Hearth_V1_Bot],
        bot: Hearth_V1_Bot?,
        draft: Binding<String>,
        reply: [Hearth_V1_Message],
        replyAuthor: Hearth_V1_Bot? = nil,
        canSend: Bool,
        error: String? = nil,
        onPick: @escaping (Hearth_V1_Bot) -> Void,
        onSend: @escaping () -> Void,
        onOpen: @escaping () -> Void
    ) {
        self.bots = bots
        self.bot = bot
        self.draft = draft
        self.reply = reply
        self.replyAuthor = replyAuthor
        self.canSend = canSend
        self.error = error
        self.onPick = onPick
        self.onSend = onSend
        self.onOpen = onOpen
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            HStack(spacing: Theme.Spacing.sm) {
                picker
                field
                Button("Send", systemImage: "arrow.up", action: onSend)
                    .buttonStyle(.borderedProminent)
                    .buttonBorderShape(.circle)
                    .labelStyle(.iconOnly)
                    .disabled(!canSend)
                    .help("Send (Return)")
            }
            if let error {
                Text(error)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(Tone.negative))
            }
            if !reply.isEmpty {
                replyArea
                    .transition(.move(edge: .top).combined(with: .opacity))
            }
        }
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: reply.isEmpty)
        .onAppear { isFocused = true }
    }

    private var picker: some View {
        Menu {
            ForEach(bots, id: \.id) { choice in
                Button(choice.name) { onPick(choice) }
            }
        } label: {
            HStack(spacing: Theme.Spacing.xs) {
                if let bot {
                    Avatar(bot: bot).accessibilityHidden(true)
                    Text(bot.name)
                        .font(Theme.callout)
                        .foregroundStyle(.primary)
                        .lineLimit(1)
                }
                Image(systemName: "chevron.down")
                    .font(Theme.caption)
                    .foregroundStyle(.secondary)
                    .accessibilityHidden(true)
            }
        }
        .menuStyle(.button)
        .buttonStyle(.plain)
        .menuIndicator(.hidden)
        .fixedSize()
        .disabled(bots.isEmpty)
        .accessibilityLabel("Ask")
        .accessibilityValue(bot?.name ?? "Nobody")
    }

    private var field: some View {
        TextField("Ask someone…", text: draft)
            .textFieldStyle(.plain)
            .font(Theme.body)
            .focused($isFocused)
            .onSubmit { if canSend { onSend() } }
            .padding(.horizontal, Theme.Spacing.sm)
            .padding(.vertical, Theme.Spacing.xs)
            .background(.quaternary, in: RoundedRectangle(cornerRadius: Theme.Radius.md))
            .accessibilityLabel(bot.map { "Ask \($0.name)" } ?? "Ask someone")
    }

    private var replyArea: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            ForEach(reply, id: \.id) { message in
                MessageRow(message: message, author: replyAuthor ?? bot)
                    .lineLimit(6)
            }
            Button("Open in Hearth", action: onOpen)
                .buttonStyle(.link)
                .font(Theme.caption)
                .accessibilityHint("Opens the conversation in the window")
        }
    }
}

extension QuickAskField {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case empty
        case typed
        case replyStreaming

        var draft: String {
            self == .typed ? "What did the pricing brief say about free plans?" : ""
        }

        var reply: [Hearth_V1_Message] {
            guard self == .replyStreaming else { return [] }
            var asked = Fixtures.sendResponse.message
            asked.content = "Remind me what the brief said about free plans?"
            var answer = Fixtures.streamingReply
            answer.content = Fixtures.replyDeltas[0]
            return [asked, answer]
        }
    }

    /// A field showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            bots: [Fixtures.chiefOfStaff, Fixtures.bot], bot: Fixtures.bot,
            draft: .constant(state.draft), reply: state.reply, canSend: state == .typed,
            onPick: { _ in }, onSend: {}, onOpen: {})
    }
}

#Preview("Empty") { QuickAskField(previewState: .empty).frame(width: 320).padding() }
#Preview("Typed") { QuickAskField(previewState: .typed).frame(width: 320).padding() }
#Preview("Reply streaming") {
    QuickAskField(previewState: .replyStreaming).frame(width: 320).padding()
}
