// The controls a question can ask with: each kind's preview states enable
// Answer where the core would take the answer and not where it would refuse
// it, the answer sent has the core's shape, and an answer reads back as the
// words an answered card shows.

import Foundation
import HearthKit
import Testing

@testable import HearthUI

@Test func everyKindEnablesAnswerOnlyWhenTheAnswerIsWithinItsLimits() {
    for state in WidgetControl.PreviewState.allCases {
        let can = state.draft.canAnswer(state.widget, prose: "")
        switch state.phase {
        case .valid, .answered: #expect(can, "\(state)")
        case .empty, .invalid: #expect(!can, "\(state)")
        }
    }
    #expect(WidgetControl.PreviewState.allCases.count == 28)
}

@Test func aControlTheOwnerMayDeclineTakesWordsInstead() {
    let multiple = WidgetSample.widget(.multiple)
    #expect(!WidgetDraft().canAnswer(multiple, prose: ""))
    #expect(WidgetDraft().canAnswer(multiple, prose: "Only Scribeworks"))
    #expect(WidgetDraft().answer(for: multiple) == nil)
    // Other chosen and left blank is no answer, so words still answer an
    // optional single choice.
    var single = WidgetSample.widget(.single)
    single.required = false
    var blankOther = WidgetDraft()
    blankOther.choosesOther = true
    #expect(blankOther.canAnswer(single, prose: "Whatever is shortest"))
    #expect(blankOther.answer(for: single) == nil)
    // A required control left empty is not answered by words.
    #expect(!WidgetDraft().canAnswer(WidgetSample.widget(.yesNo), prose: "Keep it"))
}

@Test func theAnswerHasTheCoresShape() {
    let single = WidgetSample.widget(.single)
    var draft = WidgetDraft()
    draft.choice = "slides"
    #expect(draft.answer(for: single)?.choiceID == "slides")

    var other = WidgetDraft()
    other.choosesOther = true
    other.other = " Three pages "
    let otherAnswer = other.answer(for: single)
    #expect(otherAnswer?.other == "Three pages")
    #expect(otherAnswer?.value == nil)

    var ticked = WidgetDraft()
    ticked.choices = ["scribeworks", "quillmate"]
    let multiple = WidgetSample.widget(.multiple)
    #expect(ticked.answer(for: multiple)?.choiceIds.ids == ["quillmate", "scribeworks"])
}

@Test func anAnswerReadsBackInWords() {
    for kind in WidgetSample.Kind.allCases {
        let widget = WidgetSample.widget(kind)
        let draft = WidgetSample.draft(kind, .valid)
        guard let answer = draft.answer(for: widget) else {
            Issue.record("no answer for \(kind)")
            continue
        }
        let text = WidgetDraft.text(of: answer, in: widget)
        switch kind {
        case .single: #expect(text == "Two pages")
        case .multiple: #expect(text == "Quillmate and Scribeworks")
        case .yesNo: #expect(text == "Keep it")
        case .scale: #expect(text == "4 of 5")
        case .path: #expect(text == draft.path)
        case .freeText: #expect(text == draft.text)
        case .date: #expect(text?.isEmpty == false)
        }
        // Change starts where the owner left it.
        #expect(WidgetDraft(answer: answer, to: widget).answer(for: widget) == answer)
    }
}

@Test func aDayIsSentAsItsUTCMidnightAndChecksTheRangeByDay() {
    let widget = WidgetSample.widget(.date)
    let draft = WidgetSample.draft(.date, .valid)
    let sent = draft.answer(for: widget)?.date.date
    var utc = Calendar(identifier: .gregorian)
    utc.timeZone = .gmt
    #expect(
        sent.map { utc.dateComponents([.year, .month, .day, .hour], from: $0) }
            == DateComponents(year: 2026, month: 10, day: 24, hour: 0))
    #expect(!WidgetSample.draft(.date, .invalid).isValid(for: widget))
}

@Test func aMultipleChoiceSaysItsBoundsAndTheCount() {
    var spec = Hearth_V1_MultipleChoice()
    #expect(WidgetControl.caption(spec, chosen: 2) == nil)
    spec.max = 3
    #expect(WidgetControl.caption(spec, chosen: 2) == "Choose up to 3 · 2 chosen")
    spec.min = 2
    #expect(WidgetControl.caption(spec, chosen: 4) == "Choose 2 to 3 · 4 chosen")
    let text = Hearth_V1_FreeText.with { $0.maxLength = 100 }
    #expect(WidgetControl.count(String(repeating: "a", count: 79), text) == nil)
    #expect(WidgetControl.count(String(repeating: "a", count: 80), text) == "80/100")
}
