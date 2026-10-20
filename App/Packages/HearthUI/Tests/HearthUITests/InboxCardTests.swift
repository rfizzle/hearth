// The decision bar and the Inbox cards: what each state shows and what each
// sends, for the fixture items.

import HearthDesign
import HearthKit
import Testing

@testable import HearthUI

@MainActor
@Test func theBarOffersAlwaysAllowOnlyWhereTheGrantCanStand() {
    #expect(DecisionBar.PreviewState.allCases.count == 5)
    #expect(DecisionBar.PreviewState.narrow.width < DecisionBar.PreviewState.wide.width)
    #expect(!ApprovalCard(previewState: .open).canAlwaysAllow)
    #expect(ApprovalCard(previewState: .alwaysAllowOffered).canAlwaysAllow)
    #expect(ApprovalCard(previewState: .open).item.call.name == "files.delete")
    #expect(DecisionBar.reason("  Keep it, please. \n") == "Keep it, please.")
    #expect(DecisionBar.reason(" ").isEmpty)
    #expect(ToolCallRow.symbol(for: "files.delete") == "trash")
}

@MainActor
@Test func anApprovalShowsTheCoresEffectNotTheBotsWords() {
    #expect(ApprovalCard.effect(Fixtures.approval) == Fixtures.approval.effect)
    var bare = Fixtures.approval
    bare.effect = ""
    #expect(ApprovalCard.effect(bare) == Fixtures.deleteCall.summary)
    #expect(ApprovalCard.PreviewState.closed.state == .closed("Allowed once", .positive))
    #expect(ApprovalCard.PreviewState.deciding.state.isDeciding)
}

@MainActor
@Test func aSuggestionFillsTheReplyAndABlankReplyIsNotSent() {
    let chosen = PromptCard.PreviewState.suggestionChosen
    #expect(chosen.reply == Fixtures.question.suggestedAnswers[0])
    #expect(WidgetDraft().canAnswer(nil, prose: chosen.reply))
    #expect(!WidgetDraft().canAnswer(nil, prose: " \n"))
    #expect(PromptCard.prose("  Both kinds\n") == "Both kinds")
    #expect(PromptCard(previewState: .unknownControl).item.hasWidget)
    #expect(PromptCard(previewState: .unknownControl).widget == nil)
    #expect(!PromptCard(previewState: .prose).item.hasWidget)
}

@MainActor
@Test func aFailureOffersRetryAndABudgetOffersResume() {
    #expect(!FailureCard(previewState: .open).isBudget)
    #expect(FailureCard(previewState: .budget).isBudget)
    #expect(FailureCard.PreviewState.open.item.effect == Fixtures.failedRun.failure)
    #expect(FailureCard.PreviewState.closed.state == .closed("Acknowledged", .neutral))
}

@MainActor
@Test func whileTheOwnerIsAskedSendIsHeldAndTheLineSaysSo() {
    #expect(!Composer.canSend(draft: "Add a section.", isWorking: false, isWaiting: true))
    #expect(Composer.canSend(draft: "Add a section.", isWorking: false, isWaiting: false))
    let name = Fixtures.bot.name
    let waiting = RunStatusLine.PreviewState.waiting.run
    #expect(RunStatusLine.text(run: waiting, botName: name) == "Dana is waiting for you")
    #expect(RunStatusLine.tone(run: waiting) == .attention)
    #expect(
        RunStatusLine.text(run: nil, botName: name, isWaiting: true) == "Dana is waiting for you")
    #expect(RunStatusLine.tone(run: nil, isWaiting: true) == .attention)
    #expect(RunStatusLine.tone(run: Fixtures.failedRun) == .negative)
    #expect(RunStatusLine.tone(run: Fixtures.workingRun) == .neutral)
}

@MainActor
@Test func anAnsweredRequestIsALineAndTheDecisionANotice() {
    #expect(MessageRow.PreviewState.askedApproval.message.kind == .approvalRequest)
    #expect(MessageRow.PreviewState.askedQuestion.message.kind == .question)
    #expect(MessageRow.PreviewState.decision.message.kind == .approvalDecision)
    #expect(MessageRow(previewState: .decision).isOwner)
}

@MainActor
@Test func theCardAsksWithTheControlAndGoesQuietOnceAnswered() {
    #expect(PromptCard.PreviewState.allCases.count == 17)
    #expect(PromptCard(previewState: .empty).widget != nil)
    #expect(PromptCard.answerText(PromptCard.PreviewState.answered.item) == "Two pages")
    #expect(PromptCard.answerText(PromptCard.PreviewState.answeredInWords.item) == "Both kinds")
    #expect(PromptCard(previewState: .answered).isQuiet)
    #expect(!PromptCard(previewState: .valid).isQuiet)
    // The thread's form of an answered question carries the choice.
    var message = Fixtures.dmMessages[0]
    message.kind = .question
    message.authorBotID = Fixtures.bot.id
    message.widget = WidgetSample.widget(.scale)
    message.answer = .with { $0.scale = 3 }
    let quiet = PromptCard(message: message, bot: Fixtures.bot)
    #expect(PromptCard.answerText(quiet.item) == "3 of 5")
}
