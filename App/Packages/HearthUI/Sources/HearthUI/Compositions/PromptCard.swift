// PromptCard is a question a bot asked the owner, as one card: the header
// every Inbox card has, the question in the bot's words, the control the bot
// asked for (a WidgetControl), the answers it suggested as Pill buttons that
// fill the reply, a field for the owner's own words when the control may be
// left empty (or when there is no control), and one prominent Answer, which
// Return presses on the first card of a screen. Answer is enabled once the
// control holds an answer within its limits, or the words stand in for a
// control the owner may decline; the core checks again. While the core
// answers the card is disabled with a small spinner. Answered, the card goes
// quiet: no stroke and no buttons, the control replaced by the choice as
// text, and an "Answered" Pill in the header; while the core holds the
// answer open for a short grace, Change shows and puts the control back with
// the answer in it. A control this app cannot draw says so and takes the
// answer in words. It composes CardSurface, Avatar, Pill, WidgetControl, a
// TextField and Buttons; the card is one VoiceOver container named by the
// item's title, the control keeps its system labels, and Answer says what it
// sends. The same card stands in the Inbox and in the thread.
// See docs/design.md#the-transcript and docs/model.md#owner-interaction

import HearthDesign
import HearthKit
import SwiftUI

/// A question waiting for the owner's answer, or answered.
public struct PromptCard: View {
    let item: Hearth_V1_InboxItem
    let bot: Hearth_V1_Bot
    let state: InboxCardState
    let isDefault: Bool
    let canChange: Bool
    let onAnswer: (String, Hearth_V1_WidgetAnswer?) -> Void

    @State private var reply: String
    @State private var draft: WidgetDraft
    @State private var isChanging: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// A card for `item`, which `bot` asked; `isDefault` binds Answer to
    /// Return, `canChange` offers Change on an answered card while the core
    /// still takes a new answer, and `onAnswer` gets the owner's words and
    /// the control's answer.
    public init(
        item: Hearth_V1_InboxItem,
        bot: Hearth_V1_Bot,
        state: InboxCardState,
        isDefault: Bool = true,
        canChange: Bool = false,
        reply: String = "",
        onAnswer: @escaping (String, Hearth_V1_WidgetAnswer?) -> Void
    ) {
        self.init(
            item: item, bot: bot, state: state, isDefault: isDefault, canChange: canChange,
            reply: reply, draft: WidgetDraft(), isChanging: false, onAnswer: onAnswer)
    }

    /// The answered question `message` as the thread keeps it, once its
    /// Inbox item has closed: quiet, with the owner's choice.
    public init(message: Hearth_V1_Message, bot: Hearth_V1_Bot) {
        self.init(
            item: Self.item(answered: message, title: "\(bot.name) asked"), bot: bot,
            state: .closed("Answered", .neutral), onAnswer: { _, _ in })
    }

    /// The question `message` as a closed item, carrying its control and the
    /// owner's answer.
    static func item(answered message: Hearth_V1_Message, title: String) -> Hearth_V1_InboxItem {
        .with {
            $0.kind = .question
            $0.state = .closed
            $0.botID = message.authorBotID
            $0.messageID = message.id
            $0.title = title
            $0.question = message.content
            $0.createdAt = message.createdAt
            if message.hasWidget { $0.widget = message.widget }
            if message.hasAnswer { $0.answerValue = message.answer }
        }
    }

    /// The answer as the thread's quiet card shows it for `message`.
    public static func answerText(message: Hearth_V1_Message) -> String {
        answerText(item(answered: message, title: ""))
    }

    init(
        item: Hearth_V1_InboxItem, bot: Hearth_V1_Bot, state: InboxCardState, isDefault: Bool,
        canChange: Bool, reply: String, draft: WidgetDraft, isChanging: Bool,
        onAnswer: @escaping (String, Hearth_V1_WidgetAnswer?) -> Void
    ) {
        self.item = item
        self.bot = bot
        self.state = state
        self.isDefault = isDefault
        self.canChange = canChange
        self.onAnswer = onAnswer
        _reply = State(initialValue: reply)
        _draft = State(initialValue: draft)
        _isChanging = State(initialValue: isChanging)
    }

    /// The control, when the question has one this app can draw.
    var widget: Hearth_V1_Widget? {
        item.hasWidget && item.widget.kind != nil ? item.widget : nil
    }

    /// Whether the card shows the owner's answer rather than asking.
    var isQuiet: Bool {
        if case .closed = state { return !(isChanging && canChange) }
        return false
    }

    public var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            header
            Text(item.question)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
            if isQuiet {
                answered
            } else {
                asking
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .modifier(PromptSurface(isQuiet: isQuiet))
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: isQuiet)
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: state)
        .onChange(of: state) { _, now in
            if now == .deciding { isChanging = false }
        }
        .accessibilityElement(children: .contain)
        .accessibilityLabel(item.title)
    }

    private var header: some View {
        HStack(spacing: Theme.Spacing.sm) {
            InboxCardHeader(bot: bot, date: item.createdAt.date)
            if isQuiet, case .closed(let outcome, _) = state {
                Pill(outcome, tone: .neutral)
            }
        }
    }

    // MARK: Asking

    private var asking: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            control
            if !item.suggestedAnswers.isEmpty && showsProse {
                suggestions
            }
            answerRow
        }
        .disabled(state.isDeciding)
    }

    @ViewBuilder
    private var control: some View {
        if let widget {
            WidgetControl(widget: widget, draft: $draft)
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

    /// Whether the owner may answer in words: there is no control, or it may
    /// be left empty and is not itself a field for words.
    private var showsProse: Bool {
        guard let widget else { return true }
        if case .freeText = widget.kind { return false }
        return WidgetDraft.isDeclinable(widget)
    }

    private var suggestions: some View {
        ViewThatFits(in: .horizontal) {
            HStack(spacing: Theme.Spacing.sm) { suggestionButtons }
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) { suggestionButtons }
        }
    }

    private var suggestionButtons: some View {
        ForEach(item.suggestedAnswers, id: \.self) { answer in
            SuggestionChip(answer, hint: "Puts this answer in the reply") { reply = answer }
        }
    }

    private var answerRow: some View {
        HStack(alignment: .bottom, spacing: Theme.Spacing.sm) {
            if showsProse {
                TextField(
                    widget == nil ? "Reply…" : "Or reply in your own words…", text: $reply,
                    axis: .vertical
                )
                .textFieldStyle(.roundedBorder)
                .lineLimit(1...6)
                .font(Theme.body)
                .accessibilityLabel("Reply to \(bot.name)")
                .onSubmit { if !isDefault { send() } }
            } else {
                Spacer(minLength: 0)
            }
            if state.isDeciding {
                ProgressView()
                    .controlSize(.small)
                    .accessibilityLabel("Sending your answer")
            }
            Button("Answer", action: send)
                .buttonStyle(.borderedProminent)
                .keyboardShortcut(isDefault ? KeyboardShortcut.defaultAction : nil)
                .disabled(!canAnswer)
                .accessibilityLabel(answerLabel)
                .accessibilityInputLabels(["Answer"])
        }
    }

    /// The control's answer as it would be sent, nil when it is left empty.
    var widgetAnswer: Hearth_V1_WidgetAnswer? {
        widget.flatMap { draft.answer(for: $0) }
    }

    /// Whether Answer is enabled.
    var canAnswer: Bool {
        draft.canAnswer(widget, prose: reply)
    }

    /// What VoiceOver reads for Answer: the button and what it sends.
    var answerLabel: String {
        let sent = [
            widgetAnswer.flatMap { WidgetDraft.text(of: $0, in: item.widget) }, Self.prose(reply),
        ]
        .compactMap { $0 }.filter { !$0.isEmpty }
        return sent.isEmpty ? "Answer" : "Answer: \(sent.joined(separator: ", "))"
    }

    private func send() {
        guard canAnswer else { return }
        // The card stays open until the core takes the new answer (the state
        // moves to deciding), so it does not flash the old one meanwhile.
        onAnswer(Self.prose(reply), widgetAnswer)
    }

    // MARK: Answered

    private var answered: some View {
        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
            Text(Self.answerText(item))
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .fixedSize(horizontal: false, vertical: true)
                .textSelection(.enabled)
            Spacer(minLength: Theme.Spacing.sm)
            if canChange {
                Button("Change") {
                    draft =
                        item.hasAnswerValue
                        ? WidgetDraft(answer: item.answerValue, to: item.widget) : WidgetDraft()
                    reply = item.answer
                    isChanging = true
                }
                .controlSize(.small)
                .accessibilityHint("Answer again before \(bot.name) goes on")
                .transition(.opacity)
            }
        }
    }

    /// The reply as sent: what was typed, trimmed.
    static func prose(_ reply: String) -> String {
        reply.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// The answer as an answered card shows it: the choice in words, then
    /// what the owner wrote, else the core's words for it, else that it was
    /// answered.
    public static func answerText(_ item: Hearth_V1_InboxItem) -> String {
        let value = item.answerValue
        let choice =
            item.hasAnswerValue
            ? WidgetDraft.text(of: value, in: item.widget)
                ?? (value.textForm.isEmpty ? nil : value.textForm) : nil
        let parts = [choice, item.answer].compactMap { $0 }.filter { !$0.isEmpty }
        return parts.isEmpty ? "Answered" : parts.joined(separator: "\n")
    }
}

/// The card while it asks; flat with no stroke once it is answered.
private struct PromptSurface: ViewModifier {
    let isQuiet: Bool

    func body(content: Content) -> some View {
        if isQuiet {
            content
                .padding(Theme.Spacing.md)
                .background(
                    RoundedRectangle(cornerRadius: Theme.Radius.md, style: .continuous)
                        .fill(Theme.color(.surfaceSecondary))
                )
        } else {
            content.cardSurface()
        }
    }
}
