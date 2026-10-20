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

    /// Dana, a research lead on the team hired from the first template, idle.
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
        $0.scope = .team
    }

    /// The Chief of Staff, the org-level bot hired at onboarding, idle.
    public static let chiefOfStaff = Hearth_V1_Bot.with {
        $0.id = "bot-chief-of-staff"
        $0.name = "Chief of Staff"
        $0.role = "Chief of Staff"
        $0.jobDescription =
            "What the owner asks without naming anyone comes to you first, "
            + "and you decide who does it."
        $0.tier = .balanced
        $0.status = .idle
        $0.folderPath = workspacePath + "/Chief of Staff"
        $0.dmConversationID = "conversation-chief-of-staff"
        $0.scope = .org
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

    /// A company gateway the owner added as a service of its own.
    public static let customService: Hearth_V1_ProviderAccount = .with {
        $0.provider = .openaiCompatible
        $0.state = .signedIn
        $0.profile = "a1b2c3d4"
        $0.displayName = "Acme gateway"
        $0.baseURL = "https://llm.acme.example/v1"
        $0.defaultModel = "acme-large"
        $0.label = "9c4e"
    }

    /// The services the core knows the address of, in its order: hosted
    /// services, a runtime on this Mac, then the four built-ins.
    public static let knownServices: [Hearth_V1_KnownService] = [
        known("zai", "Z.ai", "https://api.z.ai/api/paas/v4", "glm-4.6", .compatible),
        known(
            "groq", "Groq", "https://api.groq.com/openai/v1", "llama-3.3-70b-versatile", .compatible
        ),
        known("deepseek", "DeepSeek", "https://api.deepseek.com/v1", "deepseek-chat", .compatible),
        known(
            "mistral", "Mistral", "https://api.mistral.ai/v1", "mistral-large-latest", .compatible),
        known(
            "together", "Together", "https://api.together.xyz/v1",
            "meta-llama/Llama-3.3-70B-Instruct-Turbo", .compatible),
        known("ollama", "Ollama (local)", "http://localhost:11434/v1", "llama3.2", .local) {
            $0.detected = true
            $0.models = ollamaModels
        },
        known("chatgpt", "ChatGPT", "https://chatgpt.com/backend-api/codex", "", .builtin),
        known("anthropic", "Anthropic", "https://api.anthropic.com", "", .builtin),
        known("gemini", "Gemini", "https://generativelanguage.googleapis.com", "", .builtin),
        known("openrouter", "OpenRouter", "https://openrouter.ai/api/v1", "", .builtin),
    ]

    static func known(
        _ id: String, _ name: String, _ baseURL: String, _ model: String,
        _ kind: Hearth_V1_KnownServiceKind,
        _ more: (inout Hearth_V1_KnownService) -> Void = { _ in }
    ) -> Hearth_V1_KnownService {
        var service = Hearth_V1_KnownService.with {
            $0.id = id
            $0.name = name
            $0.baseURL = baseURL
            $0.helpURL = "https://example.com/\(id)"
            $0.defaultModel = model
            $0.kind = kind
        }
        more(&service)
        return service
    }

    /// The id a preview service gets: its name, lower-cased, as the core
    /// would mint a fresh one.
    static func profileID(for name: String) -> String {
        "preview-" + name.lowercased().filter { $0.isLetter || $0.isNumber }
    }

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
