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
        $0.summary = summaryText
        $0.updatedAt = time(14)
    }

    /// What Dana remembers of the part of the DM before the loaded page.
    public static let summaryText =
        "You asked Dana for a one-page brief on what our competitors charge for API access. "
        + "You want prices per month, not per call, and the free plans listed last."

    /// The brief Dana wrote, in her folder.
    public static let briefFile = Hearth_V1_FileRef.with {
        $0.path = bot.folderPath + "/Competitor pricing brief.md"
        $0.name = "Competitor pricing brief.md"
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
        dmMessage(
            1, minute: 0,
            text: "Write me a one-page brief on what our competitors charge for API access."),
        dmMessage(2, minute: 1, run: failedRun.id, kind: .system, text: failedRun.failure),
        dmMessage(3, minute: 5, text: "Try again, please."),
        dmMessage(4, minute: 6, run: workingRun.id, kind: .toolCall, text: "", tool: searchCall),
        dmMessage(
            5, minute: 6, run: workingRun.id, kind: .toolResult,
            text: "1. API pricing for AI writing tools, northwind.ai", tool: searchCall),
        dmMessage(6, minute: 7, run: workingRun.id, kind: .toolCall, text: "", tool: fetchCall),
        dmMessage(
            7, minute: 7, run: workingRun.id, kind: .toolResult,
            text: "The Team plan is $49 a month and includes 50,000 API calls.",
            tool: fetchCall),
        dmMessage(8, minute: 12, run: workingRun.id, kind: .toolCall, text: "", tool: writeCall),
        dmMessage(
            9, minute: 13, run: workingRun.id, kind: .toolResult,
            text: "Wrote Competitor pricing brief.md.", tool: writeCall, files: [briefFile]),
        streamingReply,
    ]

    /// The last message as the DM shows it mid-stream; `replyEvents` finishes it.
    public static let streamingReply = dmMessage(
        10, minute: 14, run: workingRun.id, text: replyDeltas[0], streaming: true)

    /// What PreviewClient's send answers: the owner's message and a queued run.
    public static let sendResponse = Hearth_V1_SendResponse.with {
        $0.message = dmMessage(11, minute: 20, text: "Add a section on the free plans.")
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
        $0.argumentsJson = #"{"query":"API pricing for AI writing tools"}"#
        $0.summary = "Search the web for API pricing for AI writing tools"
    }

    static let fetchCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-2"
        $0.name = "web.fetch"
        $0.argumentsJson = #"{"url":"https://northwind.ai/pricing"}"#
        $0.summary = "Read a page on northwind.ai"
    }

    static let writeCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-3"
        $0.name = "files.write"
        $0.argumentsJson = #"{"path":"Competitor pricing brief.md"}"#
        $0.summary = "Write Competitor pricing brief.md in Dana's folder"
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
