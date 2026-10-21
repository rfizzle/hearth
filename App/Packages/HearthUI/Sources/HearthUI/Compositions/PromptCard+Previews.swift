// The states PromptCard shows in previews and the gallery: a question in
// words (with a suggestion chosen, a control the app cannot draw, waiting on
// the core, answered), a single choice empty, valid, invalid, in its grace,
// changing and answered, and an open card for each other kind.

import HearthDesign
import HearthKit
import SwiftUI

extension PromptCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case prose
        case suggestionChosen
        case unknownControl
        case deciding
        case answeredInWords
        case empty
        case valid
        case invalid
        case inGrace
        case changing
        case answered
        case multipleChoice
        case freeText
        case yesNo
        case date
        case path
        case scale

        /// The control's kind, for a question that has one.
        var kind: WidgetSample.Kind? {
            switch self {
            case .prose, .suggestionChosen, .unknownControl, .deciding, .answeredInWords: nil
            case .empty, .valid, .inGrace, .changing, .answered: .single
            case .invalid, .multipleChoice: .multiple
            case .freeText: .freeText
            case .yesNo: .yesNo
            case .date: .date
            case .path: .path
            case .scale: .scale
            }
        }

        var item: Hearth_V1_InboxItem {
            var item = Fixtures.question
            switch self {
            case .unknownControl:
                item.widget = Hearth_V1_Widget()
            case .answeredInWords:
                item.answer = Fixtures.question.suggestedAnswers[1]
            default:
                break
            }
            guard let kind else { return item }
            item.id = "inbox-widget-question"
            item.question = Self.question(kind)
            item.suggestedAnswers = []
            item.widget = WidgetSample.widget(kind)
            if isAnswered, let answer = draft.answer(for: item.widget) {
                item.answerValue = answer
            }
            return item
        }

        var isAnswered: Bool {
            [.answeredInWords, .inGrace, .changing, .answered].contains(self)
        }

        var draft: WidgetDraft {
            switch self {
            case .valid, .inGrace, .changing, .answered: WidgetSample.draft(.single, .valid)
            case .invalid: WidgetSample.draft(.multiple, .invalid)
            default: WidgetDraft()
            }
        }

        var reply: String {
            switch self {
            case .suggestionChosen, .deciding: Fixtures.question.suggestedAnswers[0]
            default: ""
            }
        }

        var state: InboxCardState {
            if self == .deciding { return .deciding }
            return isAnswered ? .closed("Answered", .neutral) : .open
        }

        static func question(_ kind: WidgetSample.Kind) -> String {
            switch kind {
            case .single: "How long should the brief be?"
            case .multiple: "Which competitors should the brief cover?"
            case .freeText: "Anything about our pricing I should know?"
            case .yesNo: "There's an older brief.md in my folder. Keep it?"
            case .date: "When is the pricing meeting?"
            case .path: "Where are last quarter's sales figures?"
            case .scale: "How useful was the last brief?"
            }
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            item: state.item, bot: Fixtures.bot, state: state.state, isDefault: true,
            canChange: state == .inGrace || state == .changing, reply: state.reply,
            draft: state.draft, isChanging: state == .changing, onAnswer: { _, _ in })
    }
}

#Preview("Prose") { PromptCard(previewState: .prose).frame(width: 600).padding() }
#Preview("Suggestion chosen") {
    PromptCard(previewState: .suggestionChosen).frame(width: 600).padding()
}
#Preview("Unknown control") {
    PromptCard(previewState: .unknownControl).frame(width: 600).padding()
}
#Preview("Deciding") { PromptCard(previewState: .deciding).frame(width: 600).padding() }
#Preview("Answered in words") {
    PromptCard(previewState: .answeredInWords).frame(width: 600).padding()
}
#Preview("Empty") { PromptCard(previewState: .empty).frame(width: 600).padding() }
#Preview("Valid") { PromptCard(previewState: .valid).frame(width: 600).padding() }
#Preview("Invalid") { PromptCard(previewState: .invalid).frame(width: 600).padding() }
#Preview("In its grace") { PromptCard(previewState: .inGrace).frame(width: 600).padding() }
#Preview("Changing") { PromptCard(previewState: .changing).frame(width: 600).padding() }
#Preview("Answered") { PromptCard(previewState: .answered).frame(width: 600).padding() }
#Preview("Multiple choice") {
    PromptCard(previewState: .multipleChoice).frame(width: 600).padding()
}
#Preview("Free text") { PromptCard(previewState: .freeText).frame(width: 600).padding() }
#Preview("Yes or no") { PromptCard(previewState: .yesNo).frame(width: 600).padding() }
#Preview("Date") { PromptCard(previewState: .date).frame(width: 600).padding() }
#Preview("Path") { PromptCard(previewState: .path).frame(width: 600).padding() }
#Preview("Scale") { PromptCard(previewState: .scale).frame(width: 600).padding() }
