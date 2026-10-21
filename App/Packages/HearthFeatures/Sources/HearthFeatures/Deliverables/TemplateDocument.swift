// TemplateDocument turns the spec file of a deliverable into the text of a
// template the core keeps: the same document with a `template` header (id,
// name, when) and, for each section, `allow` (the widget kinds the section
// holds, in order of first appearance) and an empty `hint`. The widgets stay
// as the template's sample. The document is edited as parsed JSON so every
// other field is kept as the file has it; the core still checks the result
// and says in words what it refuses.
// See docs/model.md#work, docs/decisions.md (D43)

import Foundation

/// A deliverable's spec file as a template.
enum TemplateDocument {
    /// The longest id the core takes.
    static let idLimit = 40

    /// The id a name makes: lower-case letters and digits, one hyphen between
    /// runs of anything else, at most 40 characters; empty when the name has
    /// none.
    static func slug(_ name: String) -> String {
        var out = ""
        var pending = false
        let plain = name.folding(options: .diacriticInsensitive, locale: nil).lowercased()
        for scalar in plain.unicodeScalars {
            if ("a"..."z").contains(scalar) || ("0"..."9").contains(scalar) {
                if pending, !out.isEmpty { out.append("-") }
                pending = false
                out.unicodeScalars.append(scalar)
            } else {
                pending = true
            }
        }
        let cut = String(out.prefix(idLimit))
        return cut.hasSuffix("-") ? String(cut.dropLast()) : cut
    }

    /// The template text for the spec file `data`, or nil when it is not a
    /// JSON object with a list of sections.
    static func json(from data: Data, id: String, name: String, when: String) -> String? {
        guard
            var top = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
            let sections = top["sections"] as? [[String: Any]]
        else { return nil }
        top["template"] = ["id": id, "name": name, "when": when]
        top["sections"] = sections.map { section -> [String: Any] in
            var section = section
            var kinds: [String] = []
            for widget in section["widgets"] as? [[String: Any]] ?? [] {
                if let kind = widget["kind"] as? String, !kinds.contains(kind) {
                    kinds.append(kind)
                }
            }
            section["allow"] = kinds
            section["hint"] = ""
            return section
        }
        guard
            let out = try? JSONSerialization.data(
                withJSONObject: top, options: [.sortedKeys, .withoutEscapingSlashes])
        else { return nil }
        return String(bytes: out, encoding: .utf8)
    }
}
