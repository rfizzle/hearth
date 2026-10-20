// SetupScript is what the Chief of Staff says in a setup sheet, as data the
// app ships: the opening question with a worked example, which field the
// owner's paragraph fills when no draft is made, and every field in card
// order with its label, its question, a second line, an example and the
// question card's control for it. Two ship, `.project` here and `.hire` in
// SetupScript+Hire. The model never chooses the structure: the
// sheet asks the opener, then each required field the draft left empty, one
// at a time, then shows the card. Field ids are the contract's own field
// names, so a concern from the core finds its field by string. The checks on
// what is typed are the core's; a script holds none. A new sheet is a new
// static value, not a new view.
// See docs/model.md#owner-interaction and docs/design.md#forms-sheets-and-settings

import HearthKit

/// The script one setup sheet follows.
public struct SetupScript: Sendable {
    /// What the sheet sets up.
    public enum Kind: String, Sendable {
        case project
        case hire
    }

    /// The first question, answered in the owner's own words.
    public struct Opener: Sendable {
        /// "What do you want to get done?"
        public let question: String
        /// The line under the question.
        public let secondLine: String
        /// The box's placeholder: a worked paragraph.
        public let example: String
        /// The field the paragraph fills when no draft is made.
        public let seeds: String
    }

    /// One field of the card, and how the sheet asks for it.
    public struct Field: Sendable, Identifiable {
        /// The contract's name for the field.
        public let id: String
        /// The card's and the form's label: "Done when".
        public let label: String
        /// The question an asking step shows.
        public let question: String
        /// The line under the question.
        public let secondLine: String
        /// An answer that would do, shown under the control.
        public let example: String
        /// The question card's control for the field.
        public let widget: Hearth_V1_Widget
        /// The disclosure row the card shows it under, open, if any.
        public let group: String?

        /// Whether the sheet asks for it when the draft left it empty.
        public var required: Bool { widget.required }

        init(
            id: String, label: String, question: String, secondLine: String, example: String,
            widget: Hearth_V1_Widget, group: String? = nil
        ) {
            self.id = id
            self.label = label
            self.question = question
            self.secondLine = secondLine
            self.example = example
            self.widget = widget
            self.group = group
        }
    }

    public let kind: Kind
    /// The form's heading: "New Project".
    public let title: String
    /// The form's lead sentence.
    public let lead: String
    /// The line over the card.
    public let cardLine: String
    /// The card's name for VoiceOver: "Your project".
    public let cardLabel: String
    /// The primary button on the card: "Create".
    public let createLabel: String
    public let opener: Opener
    /// Every field in card order, the required ones first.
    public let fields: [Field]
    /// The defaults key that remembers "Just show me the form".
    public let preferenceKey: String
    /// Whether the opener offers the shipped role templates as chips.
    public let offersStarters: Bool
    /// The ids of the fields the card draws beside a live avatar, if any.
    public let pairedWithAvatar: [String]

    /// The field with `id`, if the script has one.
    public func field(_ id: String) -> Field? {
        fields.first { $0.id == id }
    }
}

extension SetupScript {
    /// New Project: what the owner wants to get done, then a brief.
    public static let project = SetupScript(
        kind: .project,
        title: "New Project",
        lead: "Say what done looks like and the Chief of Staff can tell when it is.",
        cardLine: "Here\u{2019}s the project as I understood it.",
        cardLabel: "Your project",
        createLabel: "Create",
        opener: Opener(
            question: "What do you want to get done?",
            secondLine: "Tell me in your own words.",
            example:
                "I want to start a newsletter for the studio. About 400 people are on the old "
                + "mailing list and there is no template yet. I'd like the first issue out "
                + "before the end of November.",
            seeds: "goal"),
        fields: [
            Field(
                id: "name", label: "Name", question: "What should we call it?",
                secondLine: "Short; it\u{2019}s the name in the sidebar.",
                example: "Launch the newsletter", widget: .line(80)),
            Field(
                id: "done_when", label: "Done when",
                question: "How will we know it\u{2019}s done?",
                secondLine: "One sentence that is true exactly when it is.",
                example: "The first issue has gone out to the list",
                widget: .line(4000)),
            Field(
                id: "goal", label: "Goal", question: "What is it for?",
                secondLine: "A sentence or two on why it matters.",
                example: "Keep the studio in touch with the people who already like its work",
                widget: .lines(4000), group: "Brief"),
            Field(
                id: "context", label: "Context",
                question: "What should the team know before they start?",
                secondLine: "What exists already, and anything they should not miss.",
                example: "About 400 subscribers on the old mailing list and no template yet",
                widget: .lines(4000), group: "Brief"),
            Field(
                id: "done_looks_like", label: "What done looks like",
                question: "What does finished look like?",
                secondLine: "The things that will exist when it is done.",
                example: "A template, a first issue and a sending schedule",
                widget: .lines(4000), group: "Brief"),
            Field(
                id: "target_date", label: "Target date", question: "When should it be done by?",
                secondLine: "Leave it off if there is no date.", example: "2026-11-30",
                widget: .day),
        ],
        preferenceKey: "setup.project.form", offersStarters: false, pairedWithAvatar: [])
}

extension Hearth_V1_Widget {
    /// A required one-line text field of at most `limit` characters.
    static func line(_ limit: Int32) -> Self {
        .with {
            $0.required = true
            $0.freeText = .with {
                $0.maxLength = limit
            }
        }
    }

    /// An optional one-line text field of at most `limit` characters.
    static func optionalLine(_ limit: Int32, placeholder: String) -> Self {
        .with {
            $0.freeText = .with {
                $0.maxLength = limit
                $0.placeholder = placeholder
            }
        }
    }

    /// A required editor of at most `limit` characters.
    static func lines(_ limit: Int32) -> Self {
        .with {
            $0.required = true
            $0.freeText = .with {
                $0.maxLength = limit
                $0.multiline = true
            }
        }
    }

    /// An optional calendar day.
    static let day = Self.with { $0.date = .init() }
}
