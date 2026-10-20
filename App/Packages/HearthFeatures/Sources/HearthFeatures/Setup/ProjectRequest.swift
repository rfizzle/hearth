// ProjectRequest turns the project script's fields into the contract's
// requests and back: a draft, a project being edited or the card's values
// become WidgetDrafts by field id, and the drafts become the create, update
// and check requests. Text travels trimmed. A target date travels as
// yyyy-mm-dd, the calendar day the picker shows on this Mac; an unset date
// travels empty.
// See docs/model.md#work

import Foundation
import HearthKit
import HearthUI

/// The project script's fields as the contract carries them.
enum ProjectRequest {
    /// The drafts for a project's name, brief and target date.
    static func drafts(
        name: String, brief: Hearth_V1_ProjectBrief, targetDate: String
    ) -> [String: WidgetDraft] {
        var drafts: [String: WidgetDraft] = [:]
        for (id, text) in [
            ("name", name), ("done_when", brief.doneWhen), ("goal", brief.goal),
            ("context", brief.context), ("done_looks_like", brief.doneLooksLike),
        ] {
            var draft = WidgetDraft()
            draft.text = text
            drafts[id] = draft
        }
        var day = WidgetDraft()
        day.date = date(from: targetDate)
        drafts["target_date"] = day
        return drafts
    }

    /// The request that creates the project the drafts hold.
    static func create(_ drafts: [String: WidgetDraft]) -> Hearth_V1_CreateProjectRequest {
        .with {
            $0.name = text("name", drafts)
            $0.brief = brief(drafts)
            $0.targetDate = day(drafts)
        }
    }

    /// The request that replaces the project with `id`'s name, brief and date.
    static func update(
        id: String, _ drafts: [String: WidgetDraft]
    ) -> Hearth_V1_UpdateProjectRequest {
        .with {
            $0.id = id
            $0.name = text("name", drafts)
            $0.brief = brief(drafts)
            $0.targetDate = day(drafts)
        }
    }

    /// The request that asks the core's rules about the drafts.
    static func check(_ drafts: [String: WidgetDraft]) -> Hearth_V1_CheckProjectRequest {
        .with {
            $0.name = text("name", drafts)
            $0.brief = brief(drafts)
            $0.targetDate = day(drafts)
        }
    }

    private static func brief(_ drafts: [String: WidgetDraft]) -> Hearth_V1_ProjectBrief {
        .with {
            $0.goal = text("goal", drafts)
            $0.context = text("context", drafts)
            $0.doneLooksLike = text("done_looks_like", drafts)
            $0.doneWhen = text("done_when", drafts)
        }
    }

    private static func text(_ id: String, _ drafts: [String: WidgetDraft]) -> String {
        trimmed(drafts[id]?.text ?? "")
    }

    private static func day(_ drafts: [String: WidgetDraft]) -> String {
        drafts["target_date"]?.date.map(text(from:)) ?? ""
    }

    static func trimmed(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func isBlank(_ text: String) -> Bool {
        trimmed(text).isEmpty
    }

    private static func formatter() -> DateFormatter {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter
    }

    /// The day `text` names (yyyy-mm-dd) on this Mac's calendar, or nil.
    static func date(from text: String) -> Date? {
        text.isEmpty ? nil : formatter().date(from: text)
    }

    /// `date`'s day as yyyy-mm-dd on this Mac's calendar.
    static func text(from date: Date) -> String {
        formatter().string(from: date)
    }
}
