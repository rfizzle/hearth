// The strict decoder behind Deliverable: every object lists the fields it
// takes, and anything else, or a null, is an error naming the field, the way
// the core refuses it. The fields per widget kind are the core's catalogue.

import Foundation

/// A coding key from any string, so an object's keys can be checked.
struct FieldKey: CodingKey {
    let stringValue: String
    var intValue: Int? { nil }

    init(_ name: String) { stringValue = name }
    init?(stringValue: String) { self.stringValue = stringValue }
    init?(intValue: Int) { nil }
}

typealias Fields = KeyedDecodingContainer<FieldKey>

extension KeyedDecodingContainer where K == FieldKey {
    /// Fails on a key outside `allowed` and on a null, naming `what` holds it.
    func only(_ allowed: Set<String>, in what: String) throws {
        for key in allKeys.sorted(by: { $0.stringValue < $1.stringValue }) {
            guard allowed.contains(key.stringValue) else {
                throw refusal("the field \"\(key.stringValue)\" is not one a \(what) takes")
            }
            if try decodeNil(forKey: key) {
                throw refusal("the \(what) has a null for \"\(key.stringValue)\"")
            }
        }
    }

    func need<T: Decodable>(_ name: String) throws -> T {
        try decode(T.self, forKey: FieldKey(name))
    }

    func maybe<T: Decodable>(_ name: String) throws -> T? {
        try decodeIfPresent(T.self, forKey: FieldKey(name))
    }

    func refusal(_ message: String) -> DecodingError {
        .dataCorrupted(.init(codingPath: codingPath, debugDescription: message))
    }
}

extension Deliverable: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        version = try fields.need("version")
        title = (try? fields.maybe("title")) ?? ""
        subtitle = try? fields.maybe("subtitle")
        template = nil
        sections = []
        let rawKind: String = try fields.need("kind")
        guard version == Self.version, let kind = Kind(rawValue: rawKind) else {
            self.kind = .report
            unsupported = "Made by a newer Hearth"
            return
        }
        self.kind = kind
        unsupported = nil
        try fields.only(
            ["version", "kind", "title", "subtitle", "template", "sections"], in: "deliverable")
        title = try fields.need("title")
        subtitle = try fields.maybe("subtitle")
        template = try fields.maybe("template")
        let fallback = kind == .dashboard ? 2 : 1
        let decoded: [Section] = try fields.need("sections")
        sections = decoded.map { section in
            var section = section
            if section.columns == 0 { section.columns = fallback }
            return section
        }
    }
}

extension Deliverable.TemplateRef: Decodable {
    public init(from decoder: any Decoder) throws {
        if let id = try? decoder.singleValueContainer().decode(String.self) {
            self.init(id: id, name: nil, when: nil)
            return
        }
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["id", "name", "when"], in: "template header")
        self.init(
            id: try fields.need("id"), name: try fields.maybe("name"),
            when: try fields.maybe("when"))
    }
}

extension Deliverable.Section: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["heading", "columns", "widgets", "allow", "hint"], in: "section")
        heading = try fields.maybe("heading")
        columns = try fields.maybe("columns") ?? 0  // the kind's default, set by the parent
        widgets = try fields.need("widgets")
        allow = try fields.maybe("allow")
        hint = try fields.maybe("hint")
    }
}

extension Deliverable.Widget: Decodable {
    /// The fields each kind takes beside kind, span and as_of: the core's
    /// catalogue.
    static let fields: [String: Set<String>] = {
        let line = "title categories series format y_label y_min y_max"
        let table: [String: String] = [
            "metric": "label value format decimals unit delta caption measure",
            "line": line,
            "bar": line + " stacked",
            "pie": "title slices format",
            "table": "title columns rows footer",
            "list": "title style items",
            "progress": "label done total caption measure",
            "light": "label tone word caption measure",
            "callout": "tone title markdown",
            "text": "markdown",
        ]
        return table.mapValues {
            Set($0.split(separator: " ").map(String.init) + ["kind", "span", "as_of"])
        }
    }()

    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        let kind: String = try fields.need("kind")
        guard let allowed = Self.fields[kind] else {
            self.init(span: 1, asOf: nil, measure: nil, content: .unsupported(kind: kind))
            return
        }
        try fields.only(allowed, in: kind)
        self.init(
            span: try fields.maybe("span") ?? 1, asOf: try fields.maybe("as_of"),
            measure: try fields.maybe("measure"),
            content: try Self.content(kind, fields))
    }

    private static func content(_ kind: String, _ fields: Fields) throws -> Deliverable.Content {
        switch kind {
        case "metric":
            .metric(
                .init(
                    label: try fields.need("label"), value: try fields.maybe("value"),
                    format: try format(fields),
                    decimals: try fields.maybe("decimals"), unit: try fields.maybe("unit"),
                    delta: try fields.maybe("delta"),
                    caption: try fields.maybe("caption")))
        case "line": .line(try chart(fields))
        case "bar": .bar(try chart(fields))
        case "pie":
            .pie(
                .init(
                    title: try fields.need("title"), slices: try fields.need("slices"),
                    format: try format(fields))
            )
        case "table":
            .table(
                .init(
                    title: try fields.maybe("title"), columns: try fields.need("columns"),
                    rows: try fields.need("rows"),
                    footer: try fields.maybe("footer")))
        case "list":
            .list(
                .init(
                    title: try fields.maybe("title"), style: try fields.maybe("style") ?? .bullet,
                    items: try fields.need("items")))
        case "progress":
            .progress(
                .init(
                    label: try fields.need("label"), done: try fields.maybe("done"),
                    total: try fields.maybe("total"),
                    caption: try fields.maybe("caption")))
        case "light":
            .light(
                .init(
                    label: try fields.need("label"), tone: try fields.maybe("tone"),
                    word: try fields.maybe("word"),
                    caption: try fields.maybe("caption")))
        case "callout":
            .callout(
                .init(
                    tone: try fields.need("tone"), title: try fields.maybe("title"),
                    markdown: try fields.need("markdown")))
        default:
            .text(markdown: try fields.need("markdown"))
        }
    }

    private static func format(_ fields: Fields) throws -> Deliverable.NumberFormat {
        try fields.maybe("format") ?? .number
    }

    private static func chart(_ fields: Fields) throws -> Deliverable.Chart {
        .init(
            title: try fields.need("title"), categories: try fields.need("categories"),
            series: try fields.need("series"),
            format: try format(fields), yLabel: try fields.maybe("y_label"),
            yMin: try fields.maybe("y_min"),
            yMax: try fields.maybe("y_max"), stacked: try fields.maybe("stacked") ?? false)
    }
}

extension Deliverable.NumberFormat: Decodable {}
extension Deliverable.Tone: Decodable {}
extension Deliverable.Align: Decodable {}
extension Deliverable.ListStyle: Decodable {}

extension Deliverable.Delta: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["value", "good"], in: "delta")
        self.init(value: try fields.need("value"), good: try fields.maybe("good"))
    }
}

extension Deliverable.Series: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["name", "values"], in: "series")
        self.init(name: try fields.need("name"), values: try fields.need("values"))
    }
}

extension Deliverable.Slice: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["label", "value"], in: "slice")
        self.init(label: try fields.need("label"), value: try fields.need("value"))
    }
}

extension Deliverable.Column: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["header", "align", "format"], in: "column")
        self.init(
            header: try fields.need("header"), align: try fields.maybe("align") ?? .left,
            format: try fields.maybe("format") ?? .number)
    }
}

extension Deliverable.Cell: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.singleValueContainer()
        if let number = try? fields.decode(Double.self) {
            self = .number(number)
        } else {
            self = .text(try fields.decode(String.self))
        }
    }
}

extension Deliverable.Item: Decodable {
    public init(from decoder: any Decoder) throws {
        let fields = try decoder.container(keyedBy: FieldKey.self)
        try fields.only(["text", "done", "detail"], in: "list item")
        self.init(
            text: try fields.need("text"), done: try fields.maybe("done") ?? false,
            detail: try fields.maybe("detail"))
    }
}
