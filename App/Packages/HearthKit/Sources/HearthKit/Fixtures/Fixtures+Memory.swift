// What slice B adds to the fixtures: Dana's notes, two handbook pages with
// the meter, two months of usage with Dana near her budget, the tiers with
// the models to pick, and what a local runtime listed.

extension Fixtures {
    /// The month the usage summary reports when asked for none.
    public static let currentMonth = "2026-10"

    /// How many notes a bot may keep.
    public static let noteCap: Int32 = 40

    /// Five things Dana remembers, the first pinned.
    public static let notes: [Hearth_V1_Note] = [
        note(1, "The owner prefers metric units and plain English.", pinned: true),
        note(2, "The owner's house is a 1930s semi.", pinned: false),
        note(3, "Grants worth checking first: the boiler upgrade scheme.", pinned: false),
        note(4, "Send briefs as Markdown in the Dana folder.", pinned: false),
        note(5, "The loft was insulated in 2019; the walls were not.", pinned: false),
    ]

    private static func note(_ number: Int, _ text: String, pinned: Bool) -> Hearth_V1_Note {
        .with {
            $0.id = "note-\(number)"
            $0.botID = bot.id
            $0.text = text
            $0.pinned = pinned
            $0.conversationID = dmConversation.id
            $0.messageID = "message-\(number)"
            $0.createdAt = time(Int64(number))
        }
    }

    /// Two pages: one for everyone, one for Dana alone.
    public static let handbookPages: [Hearth_V1_HandbookPage] = [
        .with {
            $0.id = "page-house-style"
            $0.title = "House style"
            $0.markdown = "# House style\n\nWrite plainly. Lead with the answer. Use metric units."
            $0.scope = .with { $0.everyone = true }
            $0.bytes = Int64($0.markdown.utf8.count)
            $0.updatedAt = time(10)
        },
        .with {
            $0.id = "page-sources"
            $0.title = "Sources Dana trusts"
            $0.markdown = "# Sources\n\n- Energy Saving Trust\n- Ofgem\n- The owner's own surveys"
            $0.scope = .with { $0.botID = bot.id }
            $0.bytes = Int64($0.markdown.utf8.count)
            $0.updatedAt = time(12)
        },
    ]

    /// The pages take a small share of the 24 KiB handbook budget.
    public static let handbookMeter = Hearth_V1_ContextMeter.with {
        $0.usedBytes = handbookPages.reduce(0) { $0 + $1.bytes }
        $0.capBytes = 24 * 1024
    }

    /// Usage for `month` ("" is this month): this month Dana is at 92% of her
    /// budget, last month everything stayed well under.
    public static func usageSummary(month: String) -> Hearth_V1_SummaryResponse {
        let earlier = month == "2026-09"
        return .with {
            $0.month = earlier ? month : currentMonth
            $0.org = totals(requests: earlier ? 31 : 48, micros: earlier ? 9_100_000 : 21_300_000)
            $0.bots = [
                .with {
                    $0.botID = bot.id
                    $0.totals = totals(
                        requests: earlier ? 31 : 48, micros: earlier ? 9_100_000 : 18_400_000)
                    $0.budgetCents = 2000
                }
            ]
            $0.orgBudgetCents = 5000
            $0.months = ["2026-10", "2026-09"]
        }
    }

    private static func totals(requests: Int64, micros: Int64) -> Hearth_V1_UsageTotals {
        .with {
            $0.requests = requests
            $0.steps = requests * 3
            $0.promptTokens = requests * 6200
            $0.completionTokens = requests * 540
            $0.cachedTokens = requests * 2100
            $0.costMicros = micros
            $0.priced = true
        }
    }

    /// Each tier on a model: Fast and Balanced on ChatGPT, Best on Anthropic.
    public static let tierChoices: [Hearth_V1_TierChoice] = [
        tierChoice(.fast, .chatgpt, "gpt-5-mini"),
        tierChoice(.balanced, .chatgpt, "gpt-5"),
        tierChoice(.best, .anthropic, "claude-opus-4-5"),
    ]

    private static func tierChoice(
        _ tier: Hearth_V1_Tier, _ provider: Hearth_V1_Provider, _ model: String
    ) -> Hearth_V1_TierChoice {
        .with {
            $0.tier = tier
            $0.provider = provider
            $0.model = model
        }
    }

    /// The models the owner can pick from the accounts that are signed in.
    public static let modelOptions: [Hearth_V1_ModelOption] = [
        option(.chatgpt, "", "gpt-5-mini", "ChatGPT"),
        option(.chatgpt, "", "gpt-5", "ChatGPT"),
        option(.anthropic, "", "claude-sonnet-4-5", "Anthropic"),
        option(.anthropic, "", "claude-opus-4-5", "Anthropic"),
        option(.openaiCompatible, "ollama", "llama3.2", "Ollama (local)"),
    ]

    private static func option(
        _ provider: Hearth_V1_Provider, _ profile: String, _ model: String, _ label: String
    ) -> Hearth_V1_ModelOption {
        .with {
            $0.provider = provider
            $0.profile = profile
            $0.model = model
            $0.label = label
        }
    }

    /// What Ollama listed when it answered on this Mac.
    public static let ollamaModels = ["llama3.2", "qwen2.5-coder", "mistral"]
}
