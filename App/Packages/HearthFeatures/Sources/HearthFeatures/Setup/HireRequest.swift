// HireRequest turns the hire script's fields into the contract's requests and
// back: the core's draft becomes WidgetDrafts by field id, and the drafts
// become the draft, check and create requests. Text travels trimmed. The tier
// travels as the id of its choice and starts on Balanced. The working style
// the drafter proposes closes the job description as one visible line, so
// what Create sends is text the owner has seen on the card.
// See docs/model.md#people

import Foundation
import HearthKit
import HearthUI

/// The hire script's fields as the contract carries them.
enum HireRequest {
    /// What an untouched card holds: Balanced.
    static func initialDrafts() -> [String: WidgetDraft] {
        var tier = WidgetDraft()
        tier.choice = id(of: .balanced)
        return ["tier": tier]
    }

    /// The drafts for the core's proposal; the tier only when it named one.
    static func drafts(from reply: Hearth_V1_DraftBotResponse) -> [String: WidgetDraft] {
        let style = ProjectRequest.trimmed(reply.workingStyle)
        let body = ProjectRequest.trimmed(reply.jobDescription)
        var drafts: [String: WidgetDraft] = [:]
        for (field, text) in [
            ("name", reply.name), ("role", reply.role),
            ("job_description", style.isEmpty ? body : body + "\n\nWorking style: " + style),
        ] {
            var draft = WidgetDraft()
            draft.text = text
            drafts[field] = draft
        }
        if reply.tier != .unspecified {
            var tier = WidgetDraft()
            tier.choice = id(of: reply.tier)
            drafts["tier"] = tier
        }
        return drafts
    }

    /// The request that asks the core to draft from `description`.
    static func draft(
        description: String, templateID: String?, _ drafts: [String: WidgetDraft]
    ) -> Hearth_V1_DraftBotRequest {
        .with {
            $0.description_p = description
            $0.templateID = templateID ?? ""
            $0.tier = tier(drafts)
        }
    }

    /// The request that hires the bot the drafts hold.
    static func create(
        _ drafts: [String: WidgetDraft], templateID: String?
    ) -> Hearth_V1_CreateBotRequest {
        .with {
            $0.name = text("name", drafts)
            $0.role = text("role", drafts)
            $0.jobDescription = text("job_description", drafts)
            $0.tier = tier(drafts)
            $0.avatar = text("avatar", drafts)
            $0.templateID = templateID ?? ""
        }
    }

    /// The request that asks the core's rules about the drafts.
    static func check(_ drafts: [String: WidgetDraft]) -> Hearth_V1_CheckBotRequest {
        .with {
            $0.name = text("name", drafts)
            $0.role = text("role", drafts)
            $0.jobDescription = text("job_description", drafts)
        }
    }

    /// The choice id for `tier`; Balanced for one not named.
    static func id(of tier: Hearth_V1_Tier) -> String {
        SetupScript.tiers.first { $0.tier == tier }?.id ?? "balanced"
    }

    private static func tier(_ drafts: [String: WidgetDraft]) -> Hearth_V1_Tier {
        SetupScript.tiers.first { $0.id == drafts["tier"]?.choice }?.tier ?? .balanced
    }

    private static func text(_ id: String, _ drafts: [String: WidgetDraft]) -> String {
        ProjectRequest.trimmed(drafts[id]?.text ?? "")
    }
}
