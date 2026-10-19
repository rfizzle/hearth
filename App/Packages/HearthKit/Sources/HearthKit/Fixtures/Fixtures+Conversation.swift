// The DM with Dana as the core would page it: a first run that failed, the
// owner asking again, a web search, a page read, a file written, and the reply
// still streaming. Ten messages cover every row the transcript draws in the
// slice: owner text, a system notice, tool calls, tool results (one with a
// file) and a bot message mid-stream.

import Foundation

extension Fixtures {
    /// Dana's direct-message conversation.
    public static let dmConversation = Hearth_V1_Conversation.with {
        $0.id = bot.dmConversationID
        $0.kind = .dm
        $0.botID = bot.id
        $0.title = bot.name
        $0.updatedAt = time(14)
    }

    /// The brief Dana wrote, in her folder.
    public static let briefFile = Hearth_V1_FileRef.with {
        $0.path = bot.folderPath + "/Heat pump brief.md"
        $0.name = "Heat pump brief.md"
    }

    /// The run that failed before the owner asked again.
    public static let failedRun = Hearth_V1_Run.with {
        $0.id = "run-1"
        $0.conversationID = dmConversation.id
        $0.botID = bot.id
        $0.state = .failed
        $0.failure = "ChatGPT didn't answer in time."
        $0.startedAt = time(0)
        $0.endedAt = time(1)
    }

    /// The run still writing the last message.
    public static let workingRun = Hearth_V1_Run.with {
        $0.id = "run-2"
        $0.conversationID = dmConversation.id
        $0.botID = bot.id
        $0.state = .working
        $0.steps = 3
        $0.startedAt = time(5)
    }

    /// The DM's ten messages, ascending by seq.
    public static let dmMessages: [Hearth_V1_Message] = [
        dmMessage(1, minute: 0, text: "Write me a one-page brief on heat pumps for a 1930s house."),
        dmMessage(2, minute: 1, run: failedRun.id, kind: .system, text: failedRun.failure),
        dmMessage(3, minute: 5, text: "Try again, please."),
        dmMessage(4, minute: 6, run: workingRun.id, kind: .toolCall, text: "", tool: searchCall),
        dmMessage(
            5, minute: 6, run: workingRun.id, kind: .toolResult,
            text: "1. Heat pumps in older homes, energysavingtrust.org.uk", tool: searchCall),
        dmMessage(6, minute: 7, run: workingRun.id, kind: .toolCall, text: "", tool: fetchCall),
        dmMessage(
            7, minute: 7, run: workingRun.id, kind: .toolResult,
            text: "Air-source heat pumps suit older homes once the loft and walls are insulated.",
            tool: fetchCall),
        dmMessage(8, minute: 12, run: workingRun.id, kind: .toolCall, text: "", tool: writeCall),
        dmMessage(
            9, minute: 13, run: workingRun.id, kind: .toolResult,
            text: "Wrote Heat pump brief.md.", tool: writeCall, files: [briefFile]),
        streamingReply,
    ]

    /// The last message as the DM shows it mid-stream; `replyEvents` finishes it.
    public static let streamingReply = dmMessage(
        10, minute: 14, run: workingRun.id, text: replyDeltas[0], streaming: true)

    /// What PreviewClient's send answers: the owner's message and a queued run.
    public static let sendResponse = Hearth_V1_SendResponse.with {
        $0.message = dmMessage(11, minute: 20, text: "Add a section on running costs.")
        $0.run = Hearth_V1_Run.with {
            $0.id = "run-3"
            $0.conversationID = dmConversation.id
            $0.botID = bot.id
            $0.state = .queued
            $0.startedAt = time(20)
        }
    }

    static let searchCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-1"
        $0.name = "web.search"
        $0.argumentsJson = #"{"query":"heat pumps in 1930s houses"}"#
        $0.summary = "Search the web for heat pumps in 1930s houses"
    }

    static let fetchCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-2"
        $0.name = "web.fetch"
        $0.argumentsJson = #"{"url":"https://energysavingtrust.org.uk/heat-pumps"}"#
        $0.summary = "Read a page on energysavingtrust.org.uk"
    }

    static let writeCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-3"
        $0.name = "files.write"
        $0.argumentsJson = #"{"path":"Heat pump brief.md"}"#
        $0.summary = "Write Heat pump brief.md in Dana's folder"
    }

    /// One DM message; an empty `run` makes it the owner's.
    static func dmMessage(
        _ seq: Int64, minute: Int64, run: String = "",
        kind: Hearth_V1_MessageKind = .text, text: String,
        tool: Hearth_V1_ToolCall? = nil, files: [Hearth_V1_FileRef] = [],
        streaming: Bool = false
    ) -> Hearth_V1_Message {
        .with {
            $0.id = "message-\(seq)"
            $0.conversationID = dmConversation.id
            $0.seq = seq
            $0.authorBotID = run.isEmpty ? "" : bot.id
            $0.kind = kind
            $0.content = text
            $0.createdAt = time(minute)
            $0.runID = run
            if let tool { $0.tool = tool }
            $0.files = files
            $0.streaming = streaming
        }
    }
}
