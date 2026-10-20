// WidgetDraft is the owner's answer to a question's control while it is being
// given: the option chosen, the boxes ticked, the words, the date, the path or
// the step. It is presentation state the card holds until Answer sends it.
// It mirrors the core's limits only to enable Answer (a choice that exists, a
// count within bounds, text within its length, a date in range, a path that
// is not blank); the core checks the answer again and its refusal is the one
// that counts. It also turns an answer back into the words an answered card
// shows: the label, the date, the path, "4 of 5".
// See docs/model.md#owner-interaction

import Foundation
import HearthKit

/// An answer to a question's control, as the owner gives it.
public struct WidgetDraft: Equatable, Sendable {
    /// The single choice's option id.
    public var choice: String?
    /// Whether the single choice's Other row is chosen.
    public var choosesOther = false
    /// The Other row's words.
    public var other = ""
    /// The multiple choice's ticked option ids.
    public var choices: Set<String> = []
    /// The free text.
    public var text = ""
    /// The yes-or-no answer.
    public var yes: Bool?
    /// The date, as the owner picked it on this Mac's calendar.
    public var date: Date?
    /// The chosen folder's or file's absolute path.
    public var path = ""
    /// The scale's step.
    public var scale: Int32?

    /// An untouched control.
    public init() {}

    /// The draft `answer` to `widget` was made from, so Change starts where
    /// the owner left it.
    public init(answer: Hearth_V1_WidgetAnswer, to widget: Hearth_V1_Widget) {
        switch answer.value {
        case .choiceID(let id): choice = id
        case .choiceIds(let ids): choices = Set(ids.ids)
        case .text(let text): self.text = text
        case .yes(let yes): self.yes = yes
        case .date(let stamp):
            // A day travelled as its UTC midnight; it is that day here.
            date = widget.date.withTime ? stamp.date : Self.localDay(stamp.date)
        case .path(let path): self.path = path
        case .scale(let step): scale = step
        case nil: break
        }
        if !answer.other.isEmpty {
            choosesOther = true
            other = answer.other
        }
    }

    // The catalogue's limits, as the core holds them.
    static let maxOther = 500
    static let defaultText = 1000
    static let maxPath = 4096

    /// Whether nothing has been given for `widget`'s control.
    public func isEmpty(for widget: Hearth_V1_Widget) -> Bool {
        switch widget.kind {
        case .singleChoice: choosesOther ? Self.isBlank(other) : choice == nil
        case .multipleChoice: choices.isEmpty
        case .freeText: Self.isBlank(text)
        case .yesNo: yes == nil
        case .date: date == nil
        case .path: Self.isBlank(path)
        case .scale: scale == nil
        case nil: true
        }
    }

    /// Whether what has been given is within `widget`'s limits.
    public func isValid(for widget: Hearth_V1_Widget) -> Bool {
        switch widget.kind {
        case .singleChoice(let spec):
            if choosesOther {
                return spec.allowOther && !Self.isBlank(other) && other.count <= Self.maxOther
            }
            return spec.options.contains { $0.id == choice }
        case .multipleChoice(let spec):
            let known = choices.allSatisfy { id in spec.options.contains { $0.id == id } }
            return known && choices.count >= Int(spec.min)
                && (spec.max == 0 || choices.count <= Int(spec.max))
        case .freeText(let spec):
            return text.count <= Self.limit(spec)
        case .yesNo:
            return yes != nil
        case .date(let spec):
            return date.map { Self.isInRange($0, spec) } ?? false
        case .path:
            return !Self.isBlank(path) && path.utf8.count <= Self.maxPath
        case .scale(let spec):
            return scale.map { spec.min <= $0 && $0 <= spec.max } ?? false
        case nil:
            return false
        }
    }

    /// Whether Answer is enabled for a question with `widget` and the prose
    /// reply `prose`: a control given within its limits, or one the owner may
    /// leave empty with words in its place; with no control, the words.
    public func canAnswer(_ widget: Hearth_V1_Widget?, prose: String) -> Bool {
        guard let widget, widget.kind != nil else { return !Self.isBlank(prose) }
        if isEmpty(for: widget) {
            return Self.isDeclinable(widget) && !Self.isBlank(prose)
        }
        return isValid(for: widget)
    }

    /// The answer as the core takes it, or nil when the control was left
    /// empty.
    public func answer(for widget: Hearth_V1_Widget) -> Hearth_V1_WidgetAnswer? {
        guard !isEmpty(for: widget), widget.kind != nil else { return nil }
        var answer = Hearth_V1_WidgetAnswer()
        if case .singleChoice = widget.kind, choosesOther {
            answer.other = Self.trimmed(other)
        } else {
            answer.value = value(for: widget)
        }
        return answer
    }

    private func value(for widget: Hearth_V1_Widget) -> Hearth_V1_WidgetAnswer.OneOf_Value? {
        switch widget.kind {
        case .singleChoice: choice.map { .choiceID($0) }
        case .multipleChoice(let spec):
            // In the order the bot listed them, not the order they were ticked.
            .choiceIds(.with { $0.ids = spec.options.map(\.id).filter(choices.contains) })
        case .freeText: .text(Self.trimmed(text))
        case .yesNo: yes.map { .yes($0) }
        case .date(let spec): date.map { .date(.init(date: spec.withTime ? $0 : Self.day(of: $0))) }
        case .path: .path(Self.trimmed(path))
        case .scale: scale.map { .scale($0) }
        case nil: nil
        }
    }

    /// The words an answered card shows for `answer` to `widget`: the chosen
    /// labels, the words, the date, the path, the step out of the top one.
    static func text(of answer: Hearth_V1_WidgetAnswer, in widget: Hearth_V1_Widget) -> String? {
        if !answer.other.isEmpty { return answer.other }
        switch (answer.value, widget.kind) {
        case (.choiceID(let id), .singleChoice(let spec)?):
            return spec.options.first { $0.id == id }?.label ?? id
        case (.choiceIds(let ids), .multipleChoice(let spec)?):
            let labels = ids.ids.map { id in spec.options.first { $0.id == id }?.label ?? id }
            return labels.isEmpty ? "None" : labels.formatted(.list(type: .and))
        case (.text(let text), _):
            return text
        case (.yes(let yes), .yesNo(let spec)?):
            return yes ? yesLabel(spec) : noLabel(spec)
        case (.date(let stamp), .date(let spec)?):
            return dateText(stamp.date, withTime: spec.withTime)
        case (.path(let path), _):
            return path
        case (.scale(let step), .scale(let spec)?):
            return "\(step) of \(spec.max)"
        default:
            return nil
        }
    }

    static func yesLabel(_ spec: Hearth_V1_YesNo) -> String {
        spec.yesLabel.isEmpty ? "Yes" : spec.yesLabel
    }

    static func noLabel(_ spec: Hearth_V1_YesNo) -> String {
        spec.noLabel.isEmpty ? "No" : spec.noLabel
    }

    /// The free text's limit: its own, else the catalogue's default.
    static func limit(_ spec: Hearth_V1_FreeText) -> Int {
        spec.maxLength > 0 ? Int(spec.maxLength) : defaultText
    }

    /// Whether a control may be left empty: it is not required and, for a
    /// multiple choice, asks for no minimum.
    static func isDeclinable(_ widget: Hearth_V1_Widget) -> Bool {
        if case .multipleChoice(let spec) = widget.kind, spec.min > 0 { return false }
        return !widget.required
    }

    static func trimmed(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func isBlank(_ text: String) -> Bool {
        trimmed(text).isEmpty
    }

    // A date without a time is a calendar day, which the core reads in UTC:
    // the owner's day travels as that day's UTC midnight and is shown back in
    // UTC, so it is the same day wherever the Mac is.
    private static var utc: Calendar {
        var calendar = Calendar.current
        calendar.timeZone = .gmt
        return calendar
    }

    /// The UTC midnight of `date`'s day on this Mac's calendar.
    static func day(of date: Date) -> Date {
        let parts = Calendar.current.dateComponents([.era, .year, .month, .day], from: date)
        return utc.date(from: parts) ?? date
    }

    /// The start of the local day whose UTC midnight is `stamp`: a bound of
    /// the range as the date picker takes it.
    static func localDay(_ stamp: Date) -> Date {
        let parts = utc.dateComponents([.era, .year, .month, .day], from: stamp)
        return Calendar.current.date(from: parts) ?? stamp
    }

    /// Whether `date`'s day lies within the spec's earliest and latest days.
    static func isInRange(_ date: Date, _ spec: Hearth_V1_DatePick) -> Bool {
        let day = day(of: date)
        if spec.hasEarliest, day < utc.startOfDay(for: spec.earliest.date) { return false }
        if spec.hasLatest, day > utc.startOfDay(for: spec.latest.date) { return false }
        return true
    }

    static func dateText(_ date: Date, withTime: Bool) -> String {
        if withTime {
            return date.formatted(date: .long, time: .shortened)
        }
        var style = Date.FormatStyle(date: .long, time: .omitted)
        style.timeZone = .gmt
        return date.formatted(style)
    }
}
