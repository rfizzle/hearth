// IntroductionCard is who a bot is, in the bot's own words but not as a
// message: the large avatar and name, "What I'm here for", "How to work with
// me" as three short lines with a symbol each, and "Try asking" as three
// example prompts. It sits on a CardSurface, the signal the question card
// already uses for "the app is showing you this, the bot did not say it", and
// glass stays out of it. The prompts are SuggestionChips: a press hands the
// phrase to onTry, which fills the composer and never sends. The card is
// one VoiceOver group labelled "About <name>, <role>" (the role is left out
// when it is the name, as the Chief of Staff's is) with the three headings as
// headers; the avatar and the symbols are hidden, since the line
// beside a symbol carries its meaning. It is 480 wide at most, a form column,
// because a guide set in 640 points of body text reads too long a line. The
// introduction is app-rendered text from the core; it is never in the
// transcript and never in a run's context.
// The header that opens into it, AboutRow, is in this file too.
// See docs/design.md#empty-states-and-first-run and docs/model.md#people

import HearthDesign
import HearthKit
import SwiftUI

/// A bot's introduction as a card.
public struct IntroductionCard: View {
    let introduction: Hearth_V1_Introduction
    let bot: Hearth_V1_Bot
    let onTry: (String) -> Void

    /// A card for `bot` showing `introduction`; `onTry` takes the prompt of a
    /// chip pressed.
    public init(
        introduction: Hearth_V1_Introduction, bot: Hearth_V1_Bot,
        onTry: @escaping (String) -> Void
    ) {
        self.introduction = introduction
        self.bot = bot
        self.onTry = onTry
    }

    public var body: some View {
        CardSurface {
            VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                nameBlock
                Divider()
                section("What I'm here for") {
                    Text(introduction.purpose)
                        .font(Theme.body)
                        .foregroundStyle(Theme.color(.textPrimary))
                        .fixedSize(horizontal: false, vertical: true)
                        .textSelection(.enabled)
                }
                section("How to work with me") {
                    ForEach(introduction.how, id: \.text) { line in
                        HStack(alignment: .firstTextBaseline, spacing: Theme.Spacing.sm) {
                            Image(systemName: line.symbol)
                                .symbolRenderingMode(.hierarchical)
                                .foregroundStyle(.tint)
                                .frame(width: Theme.Size.fileIcon)
                                .accessibilityHidden(true)
                            Text(line.text)
                                .font(Theme.body)
                                .foregroundStyle(Theme.color(.textPrimary))
                                .fixedSize(horizontal: false, vertical: true)
                        }
                    }
                }
                section("Try asking") {
                    ViewThatFits(in: .horizontal) {
                        HStack(spacing: Theme.Spacing.sm) { chips }
                        VStack(alignment: .leading, spacing: Theme.Spacing.sm) { chips }
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .frame(maxWidth: Theme.Size.formWidth)
        .accessibilityElement(children: .contain)
        .accessibilityLabel(
            bot.role == bot.name ? "About \(bot.name)" : "About \(bot.name), \(bot.role)")
    }

    private var nameBlock: some View {
        HStack(spacing: Theme.Spacing.md) {
            Avatar(bot: bot, size: .large)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: 0) {
                Text(bot.name)
                    .font(Theme.title)
                    .foregroundStyle(Theme.color(.textPrimary))
                if bot.role != bot.name {
                    Text(bot.role)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
            }
        }
    }

    private func section<Body: View>(
        _ heading: String, @ViewBuilder body: () -> Body
    ) -> some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
            Text(heading)
                .font(Theme.headline)
                .foregroundStyle(Theme.color(.textPrimary))
                .accessibilityAddTraits(.isHeader)
            body()
        }
    }

    private var chips: some View {
        ForEach(introduction.tryAsking, id: \.self) { prompt in
            SuggestionChip(prompt, hint: "Puts this in the message field") { onTry(prompt) }
        }
    }
}

extension IntroductionCard {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case chiefOfStaff
        case teamMember

        var bot: Hearth_V1_Bot {
            self == .chiefOfStaff ? Fixtures.chiefOfStaff : Fixtures.bot
        }
    }

    /// A card showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(introduction: state.bot.introduction, bot: state.bot, onTry: { _ in })
    }
}

/// The one-line "About <name>" header a conversation pins under its title
/// once it has messages: a DisclosureRow that opens into the same card. The
/// chevron, the click target, the keyboard and the expanded and collapsed
/// state VoiceOver reads are the system's; the card opens indented the way a
/// tool call's body does.
public struct AboutRow: View {
    @Binding var isExpanded: Bool
    let introduction: Hearth_V1_Introduction
    let bot: Hearth_V1_Bot
    let onTry: (String) -> Void

    /// A header for `bot`, open while `isExpanded`.
    public init(
        isExpanded: Binding<Bool>, introduction: Hearth_V1_Introduction, bot: Hearth_V1_Bot,
        onTry: @escaping (String) -> Void
    ) {
        _isExpanded = isExpanded
        self.introduction = introduction
        self.bot = bot
        self.onTry = onTry
    }

    public var body: some View {
        DisclosureRow(
            isExpanded: $isExpanded,
            label: {
                HStack(spacing: Theme.Spacing.sm) {
                    Avatar(bot: bot)
                        .accessibilityHidden(true)
                    Text("About \(bot.name)")
                }
            },
            content: {
                IntroductionCard(introduction: introduction, bot: bot, onTry: onTry)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(.bottom, Theme.Spacing.sm)
            })
    }
}

extension AboutRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case collapsed
        case expanded
    }

    /// A header showing `state`, for the gallery and previews. The binding is
    /// fixed, so the row does not respond to clicks.
    public init(previewState state: PreviewState) {
        self.init(
            isExpanded: .constant(state == .expanded), introduction: Fixtures.bot.introduction,
            bot: Fixtures.bot, onTry: { _ in })
    }
}

#Preview("About, collapsed") { AboutRow(previewState: .collapsed).padding() }
#Preview("About, expanded") { AboutRow(previewState: .expanded).padding() }
#Preview("Chief of Staff") { IntroductionCard(previewState: .chiefOfStaff).padding() }
#Preview("Team member") { IntroductionCard(previewState: .teamMember).padding() }
