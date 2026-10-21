// The hire script: who the owner needs and what they would do, then a card
// that reads like a row of the team list, a live avatar beside the name and
// the role. The role templates the core ships are the opener's chips, not a
// script field. The tier is a choice of three with a plain line each; it
// starts on Balanced and is never asked, so only the name, the role and the
// job description are.
// See docs/model.md#people and docs/design.md#forms-sheets-and-settings

import HearthKit

extension SetupScript {
    /// Hire someone: what the owner needs done, then who does it.
    public static let hire = SetupScript(
        kind: .hire,
        title: "Hire someone",
        lead: "Say what the job is and what good work looks like, and they can do it well.",
        cardLine: "Here\u{2019}s the colleague as I understood it.",
        cardLabel: "Your new hire",
        createLabel: "Hire",
        opener: Opener(
            question: "Who do you need, and what would they do for you?",
            secondLine: "Tell me in your own words.",
            example:
                "I need someone to keep track of our invoices. Chase the customers who pay "
                + "late, tell me each Friday what is still outstanding, and never pay anything "
                + "without asking me first.",
            seeds: "job_description"),
        fields: [
            Field(
                id: "name", label: "Name", question: "What should we call them?",
                secondLine: "A first name is fine; it\u{2019}s the name in the sidebar.",
                example: "Wren", widget: .line(80)),
            Field(
                id: "role", label: "Role", question: "What\u{2019}s the job called?",
                secondLine: "A few words, as it would read on an office door.",
                example: "Research Analyst", widget: .line(80)),
            Field(
                id: "job_description", label: "Job description",
                question: "What will they do for you?",
                secondLine: "Two or three sentences about the work, and what good work looks like.",
                example: "You look into the questions I bring you and report what you find.",
                widget: .lines(4000)),
            Field(
                id: "tier", label: "Quality", question: "How careful should the work be?",
                secondLine: "You can change this later.", example: "balanced", widget: .tier),
            Field(
                id: "avatar", label: "Avatar", question: "How should they look in the sidebar?",
                secondLine: "An emoji or a symbol name; leave it empty for initials.",
                example: "\u{1F331}",
                widget: .optionalLine(
                    80, placeholder: "An emoji or a symbol name; empty for initials")),
        ],
        preferenceKey: "setup.hire.form", offersStarters: true, pairedWithAvatar: ["name", "role"])

    /// The tier ids the tier field's choice carries, and the tiers they mean.
    static let tiers: [(id: String, tier: Hearth_V1_Tier)] = [
        ("fast", .fast), ("balanced", .balanced), ("best", .best),
    ]
}

extension Hearth_V1_Widget {
    /// The tier as a choice of three, each with a plain line.
    static let tier = Self.with {
        $0.singleChoice = .with {
            $0.options = [
                .with {
                    $0.id = "fast"
                    $0.label = "Fast"
                    $0.detail = "Quick, for routine work"
                },
                .with {
                    $0.id = "balanced"
                    $0.label = "Balanced"
                    $0.detail = "The usual choice"
                },
                .with {
                    $0.id = "best"
                    $0.label = "Best"
                    $0.detail = "Careful, for work that needs judgement"
                },
            ]
        }
    }
}
