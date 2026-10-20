// BoundValue is what a widget bound to a measure says about where its figure
// came from: the core's live read with a quiet "Live" pill, the value the file
// was written with and when, or the core's reason there is none, in the
// attention tone and in words. It also holds the few display rules the widgets
// share: numbers in the person's locale (the file keeps the core's fixed
// form), the core's units turned into what a person reads, and a spec's tone
// as the design's. The values come from the caller; nothing here reads them.
// See docs/design.md#colour

import Foundation
import HearthDesign
import HearthKit

/// Where a bound widget's figure comes from.
public enum BoundValue: Equatable, Sendable {
    /// Not bound to a measure: the file's value, said as nothing more.
    case unbound
    /// The core's live read.
    case live(Hearth_V1_MeasureValue)
    /// The file's value, read by the core at `asOf` (RFC 3339) when it wrote
    /// it; nil when the file does not say.
    case written(asOf: String?)
    /// The core has no value, for the reason it gives.
    case missing(String)

    /// The state for `widget` given the live values the caller read, keyed by
    /// measure id; nil values means nothing was read.
    public static func of(
        _ widget: Deliverable.Widget, in values: [String: Hearth_V1_MeasureValue]?
    ) -> BoundValue {
        guard let id = widget.measure else { return .unbound }
        guard let value = values?[id] else { return .written(asOf: widget.asOf) }
        return value.missing.isEmpty ? .live(value) : .missing(value.missing)
    }

    /// The words beside the figure: "Live", "As written 20 Oct", the core's
    /// reason, or nil for an unbound widget.
    public func label(timeZone: TimeZone = .current) -> String? {
        switch self {
        case .unbound: nil
        case .live: "Live"
        case .written(let asOf):
            asOf.flatMap { Self.day($0, timeZone: timeZone) }.map { "As written \($0)" }
                ?? "As written"
        case .missing(let reason): reason
        }
    }

    /// The tone the label is drawn in.
    public var tone: Tone {
        if case .missing = self { return .attention }
        return .neutral
    }

    /// Whether the figure shown is the file's, so what the file says beside
    /// it (a change, a caption that counts) still holds.
    public var showsFile: Bool {
        switch self {
        case .unbound, .written: true
        case .live, .missing: false
        }
    }

    /// The live value, when there is one.
    public var live: Hearth_V1_MeasureValue? {
        if case .live(let value) = self { return value }
        return nil
    }

    /// "20 Oct" for an RFC 3339 time, read to the minute or the second.
    static func day(_ text: String, timeZone: TimeZone) -> String? {
        let parser = DateFormatter()
        parser.locale = Locale(identifier: "en_US_POSIX")
        parser.timeZone = TimeZone(identifier: "UTC")
        let date = ["yyyy-MM-dd'T'HH:mmX", "yyyy-MM-dd'T'HH:mm:ssX"].lazy.compactMap {
            parser.dateFormat = $0
            return parser.date(from: text)
        }.first
        guard let date else { return nil }
        let out = DateFormatter()
        out.locale = Locale(identifier: "en_US_POSIX")
        out.timeZone = timeZone
        out.dateFormat = "d MMM"
        return out.string(from: date)
    }
}

/// The display rules the deliverable widgets share.
enum DeliverableDisplay {
    /// `value` in `format` for the person reading it: a currency in dollars,
    /// a percent with a % after it, an integer rounded half away from zero,
    /// a number with the decimals asked for or up to three.
    static func number(
        _ value: Double, _ format: Deliverable.NumberFormat, decimals: Int? = nil,
        locale: Locale = .current
    ) -> String {
        switch format {
        case .currency:
            return value.formatted(
                .currency(code: "USD").presentation(.narrow)
                    .precision(.fractionLength(decimals ?? 2)).locale(locale))
        case .integer:
            return value.rounded(.toNearestOrAwayFromZero).formatted(
                .number.precision(.fractionLength(0)).locale(locale))
        case .percent:
            return value.formatted(.number.precision(.fractionLength(decimals ?? 1)).locale(locale))
                + "%"
        case .number:
            let digits = decimals.map { $0...$0 } ?? 0...3
            return value.formatted(.number.precision(.fractionLength(digits)).locale(locale))
        }
    }

    /// A table cell: text as it is, a number in its column's format.
    static func cell(_ cell: Deliverable.Cell, in format: Deliverable.NumberFormat) -> String {
        switch cell {
        case .text(let text): text
        case .number(let value): number(value, format)
        }
    }

    /// A live number in the unit a person reads: micro-dollars and cents as
    /// dollars, anything else as it is.
    static func reading(_ value: Hearth_V1_MeasureValue) -> Double {
        switch value.unit {
        case "usd_micros": value.number / 1_000_000
        case "cents": value.number / 100
        default: value.number
        }
    }

    /// The design's tone for a spec's.
    static func tone(_ tone: Deliverable.Tone?) -> Tone {
        switch tone {
        case .positive: .positive
        case .attention: .attention
        case .negative: .negative
        case .neutral, nil: .neutral
        }
    }

    /// The design's tone for the core's.
    static func tone(_ tone: Hearth_V1_Tone) -> Tone {
        switch tone {
        case .positive: .positive
        case .attention: .attention
        case .negative: .negative
        default: .neutral
        }
    }

    /// Whether a link in a text block may be opened: in the system browser or
    /// mail, for http, https and mailto only.
    static func opens(_ url: URL) -> Bool {
        ["http", "https", "mailto"].contains(url.scheme?.lowercased() ?? "")
    }
}
