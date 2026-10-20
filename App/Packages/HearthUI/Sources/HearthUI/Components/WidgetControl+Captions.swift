// The words WidgetControl writes under a control: a multiple choice's bounds
// with the live count, and a free text's count once it nears its limit.
// See docs/design.md#the-transcript

import HearthKit

extension WidgetControl {
    /// A multiple choice's bounds and the live count: "Choose up to 3 · 2
    /// chosen"; nil when it has no bounds.
    nonisolated static func caption(_ spec: Hearth_V1_MultipleChoice, chosen: Int) -> String? {
        let rule: String
        switch (spec.min, spec.max) {
        case (0, 0): return nil
        case (0, let max): rule = "Choose up to \(max)"
        case (let min, 0): rule = "Choose at least \(min)"
        case (let min, let max) where min == max: rule = "Choose \(min)"
        case (let min, let max): rule = "Choose \(min) to \(max)"
        }
        return "\(rule) · \(chosen) chosen"
    }

    /// The free text's count once it nears its limit, "812/1000"; nil before.
    nonisolated static func count(_ text: String, _ spec: Hearth_V1_FreeText) -> String? {
        let limit = WidgetDraft.limit(spec)
        return text.count * 5 >= limit * 4 ? "\(text.count)/\(limit)" : nil
    }
}
