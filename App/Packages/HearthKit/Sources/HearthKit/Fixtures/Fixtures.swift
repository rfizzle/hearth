// Fixtures is the one org previews, the gallery, snapshots and store tests
// share: a named org, three role templates, one bot, the accounts behind
// sign-in and the web-search integration. The bot's DM and a scripted reply
// live in the sibling files.
// Everything is fixed, so a golden never changes because the clock moved.

import Foundation
import SwiftProtobuf

/// Fixed core state that PreviewClient serves.
public enum Fixtures {
    /// The Hearth folder every fixture path sits under.
    public static let workspacePath = "/Users/preview/Documents/Hearth"

    /// A named org with its Hearth folder.
    public static let org = Hearth_V1_Org.with {
        $0.name = "Harbor Street Studio"
        $0.workspacePath = workspacePath
    }

    /// Three templates: a research lead, a writer and a scheduler.
    public static let templates: [Hearth_V1_RoleTemplate] = [
        .with {
            $0.id = "template-research-lead"
            $0.role = "Research Lead"
            $0.summary = "Finds sources, reads them and writes short briefs."
            $0.jobDescription =
                "You research questions for the owner. Read primary sources, "
                + "say where each fact came from, and keep briefs to one page."
            $0.suggestedTier = .balanced
        },
        .with {
            $0.id = "template-writer"
            $0.role = "Writer"
            $0.summary = "Drafts letters, posts and summaries in the owner's voice."
            $0.jobDescription =
                "You write drafts for the owner. Match their tone, keep it plain, "
                + "and save each draft as a Markdown file in your folder."
            $0.suggestedTier = .best
        },
        .with {
            $0.id = "template-scheduler"
            $0.role = "Scheduler"
            $0.summary = "Keeps the calendar tidy and proposes times."
            $0.jobDescription =
                "You look after the owner's calendar. Propose times, never "
                + "accept on their behalf, and say what moved and why."
            $0.suggestedTier = .fast
        },
    ]

    /// What the model proposes for a short description of a research job.
    public static let draft = Hearth_V1_DraftBotResponse.with {
        $0.name = "Wren"
        $0.role = "Research Analyst"
        $0.jobDescription =
            "You look into the questions the owner brings you and report what you find. "
            + "You read the original sources, say where each fact came from, and keep every "
            + "brief to one page. Good work is accurate, easy to skim and honest about "
            + "what you could not confirm."
        $0.workingStyle = "Direct and brief; leads with the answer; asks before guessing"
        $0.tier = .balanced
    }

    /// Dana, a research lead hired from the first template, idle.
    public static let bot = Hearth_V1_Bot.with {
        $0.id = "bot-dana"
        $0.name = "Dana"
        $0.role = "Research Lead"
        $0.jobDescription = templates[0].jobDescription
        $0.tier = .balanced
        $0.status = .idle
        $0.avatar = ""
        $0.folderPath = workspacePath + "/Dana"
        $0.dmConversationID = "conversation-dana"
    }

    /// ChatGPT signed in, an Anthropic key stored, Gemini signed out.
    public static let accounts: [Hearth_V1_ProviderAccount] = [
        .with {
            $0.provider = .chatgpt
            $0.state = .signedIn
            $0.label = "owner@example.com"
        },
        .with {
            $0.provider = .anthropic
            $0.state = .signedIn
            $0.label = "7f3a"
        },
        .with {
            $0.provider = .gemini
            $0.state = .signedOut
        },
    ]

    /// Brave Search, the one web-search integration, with no key yet.
    public static let integrations: [Hearth_V1_IntegrationAccount] = [
        .with {
            $0.integration = .braveSearch
            $0.kind = .webSearch
            $0.name = "Brave Search"
            $0.configured = false
            $0.helpURL = "https://brave.com/search/api/"
        }
    ]

    /// The page a preview ChatGPT sign-in would open.
    public static let authorizeURL = URL(staticString: "https://auth.openai.com/oauth/authorize")

    /// 2026-10-20 09:00 UTC plus `minutes`, the clock every fixture reads.
    static func time(_ minutes: Int64) -> Google_Protobuf_Timestamp {
        Google_Protobuf_Timestamp(seconds: 1_792_486_800 + minutes * 60, nanos: 0)
    }
}

extension URL {
    /// A URL from a literal known to parse.
    fileprivate init(staticString: StaticString) {
        guard let url = URL(string: "\(staticString)") else {
            preconditionFailure("not a URL: \(staticString)")
        }
        self = url
    }
}
