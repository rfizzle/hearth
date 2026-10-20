// QuestionCard is a question a bot asked the owner: the same header as every
// Inbox card, the question in the bot's words, the answers it suggested as
// Pill buttons that fill the reply field, a field that grows to six lines, and
// Reply, the prominent button, which Return presses on the first card of a
// screen. A question may carry a control (a choice, a date); the card holds a
// slot for it under the question, and until a control is drawn there it says
// the question has one and takes the answer in words. While the core answers
// the field and Reply are disabled with a small spinner; once it has, the
// outcome replaces them as a Pill. It composes CardSurface, Avatar, Pill, a
// TextField and Buttons; the card is one VoiceOver container named by the
// item's title, and each suggestion says what pressing it does. The reply is
// presentation state and is sent as typed, trimmed.
// See docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import SwiftUI

/// A question waiting for the owner's reply.
public struct QuestionCard: View {
    let item: Hearth_V1_InboxItem
    let bot: Hearth_V1_Bot
    let state: InboxCardState
    let isDefault: Bool
    let widget: AnyView?
    let onAnswer: (String) -> Void

    @State private var reply: String
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `item`, which `bot` asked; `widget` is the question's
    /// control when one can be drawn, `isDefault` binds Reply to Return, and
    /// `onAnswer` gets the reply.
    public init(
        item: Hearth_V1_InboxItem,
        bot: Hearth_V1_Bot,
        state: InboxCardState,
        isDefault: Bool = true,
        reply: String = "",
        widget: AnyView? = nil,
        onAnswer: @escaping (String) -> Void
    ) {
        self.item = item
        self.bot = bot
        self.state = state
        self.isDefault = isDefault
        self.widget = widget
        self.onAnswer = onAnswer
        _reply = State(initialValue: reply)
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            InboxCardHeader(bot: bot, date: item.createdAt.date)
            Text(item.question)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
            control
            footer
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .cardSurface()
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: state)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(item.title)
    }

    @ViewBuilder
    private var control: some View {
        if let widget {
            widget
        } else if item.hasWidget {
            Label(
                "This question has a control the app cannot show yet. Answer in words.",
                systemImage: "info.circle"
            )
            .font(Theme.callout)
            .foregroundStyle(Theme.color(.textSecondary))
            .fixedSize(horizontal: false, vertical: true)
        }
    }

    @ViewBuilder
    private var footer: some View {
        switch state {
        case .open, .deciding:
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                if !item.suggestedAnswers.isEmpty {
                    suggestions
                }
                replyRow
            }
            .disabled(state.isDeciding)
        case .closed(let outcome, let tone):
            Pill(outcome, tone: tone)
                .transition(.blurReplace)
        }
    }

    private var suggestions: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: Theme.Spacing.sm) { suggestionButtons }
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) { suggestionButtons }
        }
    }

    private var suggestionButtons: some View {
        ForEach(item.suggestedAnswers, id: \.self) { answer in
            Button {
                reply = answer
            } label: {
                Pill(answer)
                    .frame(minHeight: Theme.Size.minTarget)
                    .contentShape(Capsule())
            }
            .buttonStyle(.plain)
            .accessibilityLabel(answer)
            .accessibilityHint("Puts this answer in the reply")
        }
    }

    private var replyRow: some View {
        HStack(alignment: .bottom, spacing: Theme.Spacing.sm) {
            TextField("Reply…", text: $reply, axis: .vertical)
                .textFieldStyle(.roundedBorder)
                .lineLimit(1...6)
                .font(Theme.body)
                .accessibilityLabel("Reply to \(bot.name)")
            if state.isDeciding {
                ProgressView()
                    .controlSize(.small)
                    .accessibilityLabel("Sending your answer")
            }
            Button("Reply") { onAnswer(Self.answer(reply)) }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(isDefault ? KeyboardShortcut.defaultAction : nil)
                .disabled(!Self.canReply(reply))
        }
    }

    /// The reply as sent: what was typed, trimmed.
    static func answer(_ reply: String) -> String {
        reply.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// Whether Reply is enabled: the reply is not blank.
    static func canReply(_ reply: String) -> Bool {
        !answer(reply).isEmpty
    }
}

extension QuestionCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case open
        case suggestionChosen
        case withControl
        case deciding
        case closed

        var item: Hearth_V1_InboxItem {
            var item = Fixtures.question
            if self == .withControl {
                item.widget = Hearth_V1_Widget()
            }
            return item
        }

        var reply: String {
            switch self {
            case .suggestionChosen, .deciding: Fixtures.question.suggestedAnswers[0]
            case .open, .withControl, .closed: ""
            }
        }

        var state: InboxCardState {
            switch self {
            case .open, .suggestionChosen, .withControl: .open
            case .deciding: .deciding
            case .closed: .closed("Answered", .positive)
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            item: state.item, bot: Fixtures.bot, state: state.state, reply: state.reply,
            onAnswer: { _ in })
    }
}

#Preview("Open") { QuestionCard(previewState: .open).frame(width: 600).padding() }
#Preview("Suggestion chosen") {
    QuestionCard(previewState: .suggestionChosen).frame(width: 600).padding()
}
#Preview("With a control") { QuestionCard(previewState: .withControl).frame(width: 600).padding() }
#Preview("Deciding") { QuestionCard(previewState: .deciding).frame(width: 600).padding() }
#Preview("Closed") { QuestionCard(previewState: .closed).frame(width: 600).padding() }
