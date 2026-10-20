// Store tests for a question asked with a control: the fixture's single
// choice lists in the Inbox, choosing an option enables Answer and sends its
// id, a multiple choice over its max keeps Answer disabled, the answered
// message stands in the thread as the owner's choice, and Change sends again
// until the core closes the item.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

@MainActor
private func loadedInbox(relay: EventRelay? = nil) async -> InboxStore {
    let store = InboxStore(
        client: PreviewClient(), relay: relay ?? EventRelay(client: PreviewClient()))
    await store.load()
    return store
}

private func changed(_ item: Hearth_V1_InboxItem) -> Hearth_V1_SubscribeResponse {
    .with { $0.event = .inboxChanged(.with { $0.item = item }) }
}

@MainActor
@Test func theFixtureWidgetQuestionListsInTheInbox() async {
    let store = await loadedInbox()
    let listed = store.items(in: .needsDecision).first { $0.id == Fixtures.widgetQuestion.id }
    #expect(listed?.widget.singleChoice.options.map(\.id) == ["one-page", "two-pages", "slides"])
    #expect(store.openItem(forMessage: Fixtures.widgetQuestionMessage.id) != nil)
    #expect(store.state(of: Fixtures.widgetQuestion) == .open)
}

@MainActor
@Test func choosingAnOptionEnablesAnswerAndSendsTheChoiceID() async {
    let store = await loadedInbox()
    let widget = Fixtures.widgetQuestion.widget
    var draft = WidgetDraft()
    // Required: words alone do not answer it.
    #expect(!draft.canAnswer(widget, prose: "Something short"))
    draft.choice = "two-pages"
    #expect(draft.canAnswer(widget, prose: ""))

    await store.answer(
        itemID: Fixtures.widgetQuestion.id, text: "", value: draft.answer(for: widget))
    let answered = store.answeredItem(Fixtures.widgetQuestion)
    #expect(answered?.answerValue.choiceID == "two-pages")
    #expect(store.state(of: Fixtures.widgetQuestion) == .closed("Answered", .positive))
    #expect(answered.map(PromptCard.answerText) == "Two pages")
}

@MainActor
@Test func aMultipleChoiceOverItsMaxKeepsAnswerDisabled() {
    let widget = Hearth_V1_Widget.with {
        $0.multipleChoice = .with {
            $0.options = ["a", "b", "c", "d"].map { id in
                .with {
                    $0.id = id
                    $0.label = id.uppercased()
                }
            }
            $0.max = 2
        }
    }
    var draft = WidgetDraft()
    draft.choices = ["a", "b"]
    #expect(draft.canAnswer(widget, prose: ""))
    draft.choices.insert("c")
    #expect(!draft.canAnswer(widget, prose: ""))
    // Words do not stand in for a control that holds an answer over its limit.
    #expect(!draft.canAnswer(widget, prose: "A, B and C"))
}

@MainActor
@Test func theAnsweredMessageShowsTheText() async {
    let relay = EventRelay(client: PreviewClient())
    let inbox = await loadedInbox(relay: relay)
    let store = ConversationStore(
        client: PreviewClient(), relay: relay, conversationID: Fixtures.dmConversation.id,
        inbox: inbox)
    await store.load()
    let asked = Fixtures.widgetQuestionMessage
    await store.handle(.with { $0.event = .messageUpserted(.with { $0.message = asked }) })

    func request() -> (Hearth_V1_Message, Hearth_V1_InboxItem?)? {
        for row in store.rows {
            if case .request(let message, let item) = row, message.id == asked.id {
                return (message, item)
            }
        }
        return nil
    }
    // Open: the card, acting through the Inbox.
    #expect(request()?.1?.id == Fixtures.widgetQuestion.id)
    #expect(request().map { RequestRow.isAnswered($0.0) } == false)

    // Answered and closed: the core's upsert carries the answer.
    var closed = Fixtures.widgetQuestion
    closed.state = .closed
    await inbox.handle(changed(closed))
    var answered = asked
    answered.answer = .with {
        $0.choiceID = "one-page"
        $0.textForm = "The owner chose: One page"
    }
    await store.handle(.with { $0.event = .messageUpserted(.with { $0.message = answered }) })
    #expect(request()?.1 == nil)
    #expect(request().map { RequestRow.isAnswered($0.0) } == true)
    #expect(request().map { PromptCard.answerText(message: $0.0) } == "One page")
}

@MainActor
@Test func changeResendsWithinTheScriptedGrace() async {
    let store = await loadedInbox()
    let item = Fixtures.widgetQuestion
    #expect(!store.canChange(item))

    await store.answer(itemID: item.id, text: "", value: .with { $0.choiceID = "one-page" })
    #expect(store.canChange(item))

    // Within the grace the owner changes the answer; it is sent again.
    await store.answer(itemID: item.id, text: "", value: .with { $0.choiceID = "slides" })
    #expect(store.answeredItem(item)?.answerValue.choiceID == "slides")
    #expect(store.canChange(item))

    // The core closes the item when the grace ends: Change goes, and an
    // answer after it is not sent.
    var closed = item
    closed.state = .closed
    await store.handle(changed(closed))
    #expect(!store.canChange(item))
    await store.answer(itemID: item.id, text: "", value: .with { $0.choiceID = "one-page" })
    #expect(store.answeredItem(item) == nil)

    // An approval, unlike a question, is never answered twice.
    await store.decide(Fixtures.approval, .allowOnce)
    #expect(!store.canChange(Fixtures.approval))
}
