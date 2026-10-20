// Deliverable is a shaped deliverable as the core writes it: the spec file
// beside the web page and the PDF, read by the app to draw it natively. The
// types mirror the core's canonical JSON field for field, and a template file
// (the same spec with a template header and, per section, the kinds it allows)
// reads as the same type. The decoder is strict: a field a widget does not
// take, a null, or a value of the wrong type is an error, as it is in the
// core. A version or a kind this Hearth does not know is not an error: the
// file decodes with `unsupported` set and one widget of a kind it does not
// know decodes as `.unsupported`, so a newer file shows a placeholder rather
// than nothing. The app never loads the web page; it draws this.
// See docs/model.md#work, docs/decisions.md (D42, D45)

import Foundation

/// A dashboard or a report a bot filled as a spec.
public struct Deliverable: Equatable, Sendable {
    /// Report or dashboard; a report defaults to one column, a dashboard to
    /// two.
    public enum Kind: String, Equatable, Sendable {
        case report
        case dashboard
    }

    /// The version of the spec this Hearth reads.
    public static let version = 1

    public internal(set) var version: Int
    /// The kind; `.report` when the file is `unsupported`.
    public internal(set) var kind: Kind
    public internal(set) var title: String
    public internal(set) var subtitle: String?
    /// The template it was made from: an id in a deliverable, the whole
    /// header in a template file.
    public internal(set) var template: TemplateRef?
    /// The sections in order; empty when the file is `unsupported`.
    public internal(set) var sections: [Section]
    /// Why this Hearth cannot draw the file (a newer version or kind), or nil.
    public internal(set) var unsupported: String?
}

extension Deliverable {
    /// The template a deliverable names, or a template file's header.
    public struct TemplateRef: Equatable, Sendable {
        public let id: String
        public let name: String?
        public let when: String?
    }

    /// A grid of widgets under an optional heading.
    public struct Section: Equatable, Sendable {
        public internal(set) var heading: String?
        /// How many columns the grid has, the kind's default when the file
        /// leaves it out.
        public internal(set) var columns: Int
        public internal(set) var widgets: [Widget]
        /// In a template file, the widget kinds the section takes.
        public internal(set) var allow: [String]?
        /// In a template file, what the section is for.
        public internal(set) var hint: String?
    }

    /// One widget: its content by kind, how many columns it spans, and the
    /// measure it is bound to with the time the core read it.
    public struct Widget: Equatable, Sendable {
        public internal(set) var span: Int
        /// When the core read the bound value, RFC 3339 in UTC to the minute.
        public internal(set) var asOf: String?
        /// The measure id the value is bound to, on a metric, progress or
        /// light.
        public internal(set) var measure: String?
        public internal(set) var content: Content
    }

    /// What a widget shows, by kind.
    public enum Content: Equatable, Sendable {
        case metric(Metric)
        case line(Chart)
        case bar(Chart)
        case pie(Pie)
        case table(Table)
        case list(List)
        case progress(Progress)
        case light(Light)
        case callout(Callout)
        case text(markdown: String)
        /// A kind this Hearth does not know, with the kind's name.
        case unsupported(kind: String)
    }

    /// How a number is written.
    public enum NumberFormat: String, Equatable, Sendable {
        case number
        case integer
        case currency
        case percent
    }

    /// A status in words and a tone, as a light or a callout carries it.
    public enum Tone: String, Equatable, Sendable {
        case neutral
        case positive
        case attention
        case negative
    }

    /// One figure with an optional change beside it.
    public struct Metric: Equatable, Sendable {
        public let label: String
        /// The file's value; nil only on a bound metric the core could not
        /// fill.
        public let value: Double?
        public let format: NumberFormat
        public let decimals: Int?
        public let unit: String?
        public let delta: Delta?
        public let caption: String?

        public init(
            label: String, value: Double?, format: NumberFormat = .number, decimals: Int? = nil,
            unit: String? = nil, delta: Delta? = nil, caption: String? = nil
        ) {
            self.label = label
            self.value = value
            self.format = format
            self.decimals = decimals
            self.unit = unit
            self.delta = delta
            self.caption = caption
        }
    }

    /// The change a metric shows; `good` says whether it is a good one, so a
    /// falling cost can read as good.
    public struct Delta: Equatable, Sendable {
        public let value: Double
        public let good: Bool?

        public init(value: Double, good: Bool?) {
            self.value = value
            self.good = good
        }
    }

    /// A line or a bar chart: one value per category in each series.
    public struct Chart: Equatable, Sendable {
        public let title: String
        public let categories: [String]
        public let series: [Series]
        public let format: NumberFormat
        public let yLabel: String?
        public let yMin: Double?
        public let yMax: Double?
        /// Bars stacked rather than side by side; always false on a line.
        public let stacked: Bool

        public init(
            title: String, categories: [String], series: [Series], format: NumberFormat = .number,
            yLabel: String? = nil, yMin: Double? = nil, yMax: Double? = nil, stacked: Bool = false
        ) {
            self.title = title
            self.categories = categories
            self.series = series
            self.format = format
            self.yLabel = yLabel
            self.yMin = yMin
            self.yMax = yMax
            self.stacked = stacked
        }
    }

    /// One named run of values.
    public struct Series: Equatable, Sendable {
        public let name: String
        public let values: [Double]

        public init(name: String, values: [Double]) {
            self.name = name
            self.values = values
        }
    }

    /// A pie of two to eight slices.
    public struct Pie: Equatable, Sendable {
        public let title: String
        public let slices: [Slice]
        public let format: NumberFormat
    }

    /// One wedge of a pie.
    public struct Slice: Equatable, Sendable {
        public let label: String
        public let value: Double
    }

    /// Rows of cells under column headers, with an optional footer row.
    public struct Table: Equatable, Sendable {
        public let title: String?
        public let columns: [Column]
        public let rows: [[Cell]]
        public let footer: [Cell]?
    }

    /// A table column's header, alignment and number format.
    /// Where a table column's cells sit.
    public enum Align: String, Equatable, Sendable {
        case left
        case center
        case right
    }

    public struct Column: Equatable, Sendable {
        public let header: String
        public let align: Align
        public let format: NumberFormat
    }

    /// A table cell: text or a number.
    public enum Cell: Equatable, Sendable {
        case text(String)
        case number(Double)
    }

    /// A list of lines.
    /// How a list marks its lines.
    public enum ListStyle: String, Equatable, Sendable {
        case bullet
        case numbered
        case check
    }

    public struct List: Equatable, Sendable {
        public let title: String?
        public let style: ListStyle
        public let items: [Item]

        public init(title: String?, style: ListStyle, items: [Item]) {
            self.title = title
            self.style = style
            self.items = items
        }
    }

    /// One line of a list; `done` is read only in a check list.
    public struct Item: Equatable, Sendable {
        public let text: String
        public let done: Bool
        public let detail: String?
    }

    /// Done out of total.
    public struct Progress: Equatable, Sendable {
        public let label: String
        public let done: Double?
        public let total: Double?
        public let caption: String?

        public init(label: String, done: Double?, total: Double?, caption: String? = nil) {
            self.label = label
            self.done = done
            self.total = total
            self.caption = caption
        }
    }

    /// A status light: a tone and the word that says it.
    public struct Light: Equatable, Sendable {
        public let label: String
        public let tone: Tone?
        public let word: String?
        public let caption: String?

        public init(label: String, tone: Tone?, word: String?, caption: String? = nil) {
            self.label = label
            self.tone = tone
            self.word = word
            self.caption = caption
        }
    }

    /// A note in a tone, its text in Markdown.
    public struct Callout: Equatable, Sendable {
        public let tone: Tone
        public let title: String?
        public let markdown: String

        public init(tone: Tone, title: String?, markdown: String) {
            self.tone = tone
            self.title = title
            self.markdown = markdown
        }
    }
}

extension Deliverable {
    /// Reads a spec or a template file, strictly.
    public init(json data: Data) throws {
        self = try JSONDecoder().decode(Deliverable.self, from: data)
    }
}
