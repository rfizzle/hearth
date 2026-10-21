// ConversationRows turns the loaded messages into the rows a transcript
// draws. Each tool call carries its result, matched by call id, and a result
// whose call is not loaded stands alone; a request carries its Inbox item
// while that is open. Once a run has ended, its tool calls and its interim
// texts fold into one worked row, and its last text stays outside as the
// answer; a run still working is not folded, so its rows show as they arrive
// and fold when it ends; with no run reported nothing is known to have
// ended, so nothing folds. The duration is known only for the latest run, whose
// times the store holds; an older run shows none rather than a guess.
// See docs/design.md#the-transcript

import Foundation
import HearthKit
import HearthUI

enum ConversationRows {
    /// The rows for `messages`, with `run` as the conversation's latest run
    /// and `item` finding the open Inbox item for a request's message.
    static func build(
        messages: [Hearth_V1_Message], run: Hearth_V1_Run?,
        item: (String) -> Hearth_V1_InboxItem?
    ) -> [ConversationStore.Row] {
        let results = Dictionary(
            messages.filter { $0.kind == .toolResult }.map { ($0.tool.callID, $0) },
            uniquingKeysWith: { _, last in last })
        let calls = Set(messages.filter { $0.kind == .toolCall }.map(\.tool.callID))
        let runs = Dictionary(
            messages.map { ($0.id, $0.runID) }, uniquingKeysWith: { first, _ in first })
        let working = run.map { isWorking($0) } ?? false
        let answers = answerIDs(messages)

        // The rows as they arrive, then the finished runs' members pulled out
        // into one row at the place of the first of them.
        var members: [String: [ConversationStore.Row]] = [:]
        var firsts: [String: Int] = [:]
        var out: [ConversationStore.Row?] = []
        for message in messages {
            let row: ConversationStore.Row?
            switch message.kind {
            case .toolCall:
                row = .tool(
                    call: message.tool, id: message.id, result: results[message.tool.callID])
            case .toolResult:
                row =
                    calls.contains(message.tool.callID)
                    ? nil : .tool(call: message.tool, id: message.id, result: message)
            case .approvalRequest, .question:
                row = .request(message, item: item(message.id))
            default:
                row = .message(message)
            }
            guard let row else { continue }
            let runID = runs[message.id] ?? ""
            let folds =
                run != nil && !runID.isEmpty && !(working && run?.id == runID)
                && (message.kind == .toolCall || message.kind == .toolResult
                    || (message.kind == .text && !message.authorBotID.isEmpty
                        && answers[runID] != message.id))
            guard folds else {
                out.append(row)
                continue
            }
            if firsts[runID] == nil {
                firsts[runID] = out.count
                out.append(nil)
            }
            members[runID, default: []].append(row)
        }
        for (runID, index) in firsts {
            out[index] = .worked(
                id: "worked-\(runID)", duration: duration(of: runID, run: run),
                steps: steps(members[runID] ?? []))
        }
        return out.compactMap { $0 }
    }

    /// The id of each run's last bot text, which stays out of the fold.
    private static func answerIDs(_ messages: [Hearth_V1_Message]) -> [String: String] {
        var answers: [String: String] = [:]
        for message in messages
        where message.kind == .text && !message.authorBotID.isEmpty && !message.runID.isEmpty {
            answers[message.runID] = message.id
        }
        return answers
    }

    private static func isWorking(_ run: Hearth_V1_Run) -> Bool {
        switch run.state {
        case .queued, .working, .waitingOwner: true
        default: false
        }
    }

    private static func duration(of runID: String, run: Hearth_V1_Run?) -> TimeInterval? {
        guard let run, run.id == runID, run.hasStartedAt, run.hasEndedAt else { return nil }
        let seconds = run.endedAt.date.timeIntervalSince(run.startedAt.date)
        return seconds >= 0 ? seconds : nil
    }

    /// The members in order: a text is narration, and the calls between two
    /// texts are one burst.
    private static func steps(_ members: [ConversationStore.Row]) -> [TraceStep] {
        var steps: [TraceStep] = []
        var burst: [TracedCall] = []
        func flush() {
            if !burst.isEmpty { steps.append(.tools(burst)) }
            burst = []
        }
        for member in members {
            switch member {
            case .tool(let call, let id, let result):
                burst.append(TracedCall(call: call, id: id, result: result))
            case .message(let message):
                flush()
                steps.append(.narration(message))
            default:
                break
            }
        }
        flush()
        return steps
    }
}
