// The grouping of a finished run into one worked row: a run with no tools, a
// run with tools only, interleaved texts and tools, and a run still working.

import Foundation
import HearthKit
import HearthUI
import Testing

@testable import HearthFeatures

private func message(
    _ seq: Int64, run: String = "r1", kind: Hearth_V1_MessageKind = .text, text: String = "",
    call: String = "", name: String = "web.search"
) -> Hearth_V1_Message {
    .with {
        $0.id = "m\(seq)"
        $0.seq = seq
        $0.kind = kind
        $0.content = text
        $0.runID = run
        $0.authorBotID = run.isEmpty ? "" : "bot"
        if !call.isEmpty {
            $0.tool = .with {
                $0.callID = call
                $0.name = name
            }
        }
    }
}

private func run(
    _ state: Hearth_V1_RunState, id: String = "r1", seconds: Int? = 252
) -> Hearth_V1_Run {
    .with {
        $0.id = id
        $0.state = state
        if let seconds {
            $0.startedAt = .with { $0.seconds = 1000 }
            $0.endedAt = .with { $0.seconds = 1000 + Int64(seconds) }
        }
    }
}

private func rows(
    _ messages: [Hearth_V1_Message], _ run: Hearth_V1_Run?
) -> [ConversationStore.Row] {
    ConversationRows.build(messages: messages, run: run) { _ in nil }
}

@Test func aRunWithNoToolsIsNotFolded() {
    let messages = [message(1, text: "Here it is")]
    #expect(rows(messages, run(.done)).map(\.id) == ["m1"])
    #expect(
        rows(messages + [message(2, kind: .toolCall, call: "c")], nil).map(\.id) == ["m1", "m2"])
}

@Test func aRunWithToolsOnlyFoldsIntoOneRowWithoutAnAnswer() {
    let messages = [
        message(1, kind: .toolCall, call: "c1"),
        message(2, kind: .toolResult, call: "c1"),
        message(3, kind: .toolCall, call: "c2", name: "web.fetch"),
    ]
    let built = rows(messages, run(.done))
    #expect(built.map(\.id) == ["worked-r1"])
    guard case .worked(_, let duration, let steps) = built[0] else {
        Issue.record("not a worked row")
        return
    }
    #expect(duration == 252)
    #expect(steps.count == 1)
    guard case .tools(let calls) = steps[0] else {
        Issue.record("not a burst")
        return
    }
    #expect(calls.map(\.id) == ["m1", "m3"])
    #expect(calls[0].result?.id == "m2")
}

@Test func interleavedTextsAndToolsFoldAroundTheLastText() {
    let messages = [
        message(1, run: "", text: "Please"),
        message(2, text: "Looking"),
        message(3, kind: .toolCall, call: "c1"),
        message(4, kind: .toolResult, call: "c1"),
        message(5, kind: .toolCall, call: "c2"),
        message(6, text: "Now writing"),
        message(7, kind: .toolCall, call: "c3", name: "files.write"),
        message(8, text: "Done"),
    ]
    let built = rows(messages, run(.done, id: "older"))
    #expect(built.map(\.id) == ["m1", "worked-r1", "m8"])
    guard case .worked(_, let duration, let steps) = built[1] else {
        Issue.record("not a worked row")
        return
    }
    #expect(duration == nil)
    #expect(steps.map(\.id) == ["m2", "m3", "m6", "m7"])
}

@Test func aRunStillWorkingShowsItsRowsAsTheyArrive() {
    let messages = [
        message(1, text: "Looking"),
        message(2, kind: .toolCall, call: "c1"),
        message(3, text: "Writing"),
    ]
    #expect(rows(messages, run(.working)).map(\.id) == ["m1", "m2", "m3"])
    #expect(rows(messages, run(.working, id: "other")).map(\.id) == ["worked-r1", "m3"])
    #expect(rows(messages, run(.done)).map(\.id) == ["worked-r1", "m3"])
}
