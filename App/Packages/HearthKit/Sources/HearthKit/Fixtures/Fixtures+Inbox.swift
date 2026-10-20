// What waits on the owner, as the core would list it: an approval to delete a
// file, a question with two suggested answers and a failure, and the grants
// behind the Access sheet. The approval has its thread messages so a screen
// can draw the request in the transcript and in the Inbox from one source.

import Foundation

extension Fixtures {
    /// The delete Dana asked to make.
    public static let deleteCall = Hearth_V1_ToolCall.with {
        $0.callID = "call-4"
        $0.name = "files.delete"
        $0.argumentsJson = #"{"path":"Heat pump brief.md"}"#
        $0.summary = "Delete brief.md from Dana's folder"
    }

    /// The thread message the approval stands behind.
    public static let approvalRequest = dmMessage(
        11, minute: 16, run: workingRun.id, kind: .approvalRequest,
        text: "Dana wants to delete brief.md", tool: deleteCall)

    /// The owner's decision as the thread records it.
    public static let approvalDecision = dmMessage(
        12, minute: 17, kind: .approvalDecision, text: "Allowed once", tool: deleteCall)

    /// What the delete printed once it ran.
    public static let deleteResult = dmMessage(
        13, minute: 17, run: workingRun.id, kind: .toolResult,
        text: "Deleted brief.md.", tool: deleteCall)

    /// An open approval to delete `brief.md`.
    public static let approval = Hearth_V1_InboxItem.with {
        $0.id = "inbox-approval"
        $0.kind = .approval
        $0.state = .open
        $0.botID = bot.id
        $0.conversationID = dmConversation.id
        $0.runID = workingRun.id
        $0.messageID = approvalRequest.id
        $0.title = "Dana wants to delete a file"
        $0.effect = "Deletes brief.md from Dana's folder. This cannot be undone."
        $0.call = deleteCall
        $0.createdAt = time(16)
    }

    /// An open question with two suggested answers.
    public static let question = Hearth_V1_InboxItem.with {
        $0.id = "inbox-question"
        $0.kind = .question
        $0.state = .open
        $0.botID = bot.id
        $0.conversationID = dmConversation.id
        $0.runID = "run-1"
        $0.messageID = "message-20"
        $0.title = "Dana has a question"
        $0.question = "Should the brief cover air-source pumps only, or ground-source too?"
        $0.suggestedAnswers = ["Air-source only", "Both kinds"]
        $0.createdAt = time(10)
    }

    /// An open notice that a run failed.
    public static let failure = Hearth_V1_InboxItem.with {
        $0.id = "inbox-failure"
        $0.kind = .failure
        $0.state = .open
        $0.botID = bot.id
        $0.conversationID = dmConversation.id
        $0.runID = failedRun.id
        $0.messageID = "message-2"
        $0.title = "Dana's run failed"
        $0.effect = failedRun.failure
        $0.createdAt = time(1)
    }

    /// The Inbox PreviewClient lists: the approval, the question and the failure.
    public static let inbox: [Hearth_V1_InboxItem] = [approval, question, failure]

    /// Dana's grants at the defaults: reading allowed, everything else asks,
    /// deleting never standing; each file capability sits on her own folder.
    public static let grants: [Hearth_V1_Grant] = [
        grant(.filesRead, .allowed, "Read files", folder: bot.folderPath, canAllow: true),
        grant(.filesWrite, .ask, "Write files", folder: bot.folderPath, canAllow: true),
        grant(.filesDelete, .ask, "Delete files", folder: bot.folderPath, canAllow: false),
        grant(.webFetch, .ask, "Read web pages", canAllow: true),
        grant(.webSearch, .ask, "Search the web", canAllow: true),
    ]

    private static func grant(
        _ capability: Hearth_V1_Capability, _ mode: Hearth_V1_GrantMode, _ title: String,
        folder: String = "", canAllow: Bool
    ) -> Hearth_V1_Grant {
        .with {
            $0.capability = capability
            $0.mode = mode
            $0.folder = folder
            $0.title = title
            $0.canAllow = canAllow
            $0.ownFolder = !folder.isEmpty
        }
    }
}
