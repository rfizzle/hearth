// RunTrace is a finished run folded into one quiet line, "Worked for 4m 12s",
// closed by default. Open, it shows the bot's narration as plain text and each
// unbroken burst of tool calls between two texts as one line with one icon,
// counted from the call names by rule ("Read 3 files, searched the web, wrote
// 1 file"), never written by a model; the files a burst produced stay under
// its line. It is a DisclosureRow, so keyboard and VoiceOver behaviour is the
// system's, and opening and closing move with the layout move so the answer
// below slides rather than jumps. A run with no known times reads "Worked on
// this".
// See docs/design.md#the-transcript and docs/design.md#motion

import HearthDesign
import HearthKit
import SwiftUI

/// One tool call of a run with its result, once it has arrived.
public struct TracedCall: Equatable, Identifiable, Sendable {
    public let call: Hearth_V1_ToolCall
    public let id: String
    public let result: Hearth_V1_Message?

    public init(call: Hearth_V1_ToolCall, id: String, result: Hearth_V1_Message?) {
        self.call = call
        self.id = id
        self.result = result
    }
}

/// One step of a folded run: something the bot said, or a burst of calls.
public enum TraceStep: Equatable, Identifiable, Sendable {
    case narration(Hearth_V1_Message)
    case tools([TracedCall])

    public var id: String {
        switch self {
        case .narration(let message): message.id
        case .tools(let calls): calls.first?.id ?? ""
        }
    }
}

/// A folded run.
public struct RunTrace: View {
    let duration: TimeInterval?
    let steps: [TraceStep]

    @State private var isExpanded: Bool
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    /// Under the bot's name: the avatar's width and the gap after it.
    @ScaledMetric(relativeTo: .body) private var indent: CGFloat =
        Theme.Size.avatar + Theme.Spacing.sm

    /// A line for `steps`; `duration` is nil when the run's times are not
    /// known.
    public init(duration: TimeInterval?, steps: [TraceStep], isExpanded: Bool = false) {
        self.duration = duration
        self.steps = steps
        _isExpanded = State(initialValue: isExpanded)
    }

    public var body: some View {
        DisclosureRow(isExpanded: $isExpanded) {
            Text(Self.title(duration: duration))
        } content: {
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                ForEach(steps) { step in
                    switch step {
                    case .narration(let message):
                        Text(message.content)
                            .font(Theme.callout)
                            .foregroundStyle(Theme.color(.textSecondary))
                            .textSelection(.enabled)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    case .tools(let calls):
                        burst(calls)
                    }
                }
            }
            .padding(.vertical, Theme.Spacing.xs)
        }
        .padding(.leading, indent)
        .frame(maxWidth: Theme.Size.readableWidth, alignment: .leading)
        .animation(Theme.motion(Theme.move, reduceMotion: reduceMotion), value: isExpanded)
    }

    private func burst(_ calls: [TracedCall]) -> some View {
        let names = calls.map(\.call.name)
        return VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
            HStack(spacing: Theme.Spacing.sm) {
                Image(systemName: ToolBurst.symbol(names: names))
                    .accessibilityHidden(true)
                Text(ToolBurst.line(names: names))
            }
            .font(Theme.callout)
            .foregroundStyle(Theme.color(.textSecondary))
            TranscriptFiles(files: calls.flatMap { $0.result?.files ?? [] })
        }
    }

    /// "Worked for 4m 12s", or "Worked on this" with no time.
    static func title(duration: TimeInterval?) -> String {
        guard let duration else { return "Worked on this" }
        let seconds = Int(duration.rounded())
        let (hours, minutes, rest) = (seconds / 3600, seconds % 3600 / 60, seconds % 60)
        if hours > 0 { return "Worked for \(hours)h \(minutes)m" }
        if minutes > 0 { return "Worked for \(minutes)m \(rest)s" }
        return "Worked for \(rest)s"
    }
}

/// What a burst of tool calls reads as, by rule from the call names.
enum ToolBurst {
    /// A verb and its noun in the singular and the plural.
    struct Words {
        let verb: String
        let one: String
        let many: String

        init(_ verb: String, _ one: String, _ many: String) {
            self.verb = verb
            self.one = one
            self.many = many
        }
    }

    /// The kinds of call the core registers, in the order a line names them.
    enum Kind: CaseIterable {
        case readFile, search, readPage, writeFile, document, convert, deleteFile
        case remember, ask, propose

        /// The call names the core registers, by kind.
        private static let byName: [String: Kind] = [
            "files.read": .readFile, "web.search": .search, "web.fetch": .readPage,
            "files.write": .writeFile, "deliverables.write": .document,
            "files.render": .convert, "files.delete": .deleteFile, "remember": .remember,
            "ask_owner": .ask, "propose_project": .propose,
        ]

        init?(name: String) {
            guard let kind = Self.byName[name] else { return nil }
            self = kind
        }

        var symbol: String {
            switch self {
            case .readFile: "doc.text"
            case .search: "magnifyingglass"
            case .readPage: "globe"
            case .writeFile: "doc.badge.plus"
            case .document: "doc.richtext"
            case .convert: "arrow.triangle.2.circlepath"
            case .deleteFile: "trash"
            case .remember: "brain"
            case .ask: "questionmark.bubble"
            case .propose: "folder.badge.plus"
            }
        }

        /// The verb and the noun in the singular and the plural.
        private var words: Words {
            switch self {
            case .readFile: Words("read", "file", "files")
            case .search: Words("searched", "the web", "the web")
            case .readPage: Words("read", "page", "pages")
            case .writeFile: Words("wrote", "file", "files")
            case .document: Words("made", "document", "documents")
            case .convert: Words("converted", "file", "files")
            case .deleteFile: Words("deleted", "file", "files")
            case .remember: Words("made", "note", "notes")
            case .ask: Words("asked", "question", "questions")
            case .propose: Words("proposed", "project", "projects")
            }
        }

        func phrase(_ count: Int) -> String {
            if self == .search {
                return count == 1 ? "searched the web" : "searched the web \(count) times"
            }
            let words = words
            return count == 1
                ? "\(words.verb) 1 \(words.one)" : "\(words.verb) \(count) \(words.many)"
        }
    }

    /// The burst's line: a count per kind in a fixed order, then the calls the
    /// rule does not know as "used <n> tools".
    static func line(names: [String]) -> String {
        let kinds = names.map(Kind.init(name:))
        var phrases = Kind.allCases.compactMap { kind -> String? in
            let count = kinds.filter { $0 == kind }.count
            return count == 0 ? nil : kind.phrase(count)
        }
        let unknown = kinds.filter { $0 == nil }.count
        if unknown > 0 { phrases.append(unknown == 1 ? "used 1 tool" : "used \(unknown) tools") }
        let joined = phrases.joined(separator: ", ")
        return joined.prefix(1).uppercased() + joined.dropFirst()
    }

    /// The one icon: the first kind the line names, else a gear.
    static func symbol(names: [String]) -> String {
        let kinds = Set(names.compactMap(Kind.init(name:)))
        return Kind.allCases.first { kinds.contains($0) }?.symbol ?? "gearshape"
    }
}

extension RunTrace {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case folded
        case open
        case noTimes

        /// A search and a page read, a word, then a file written.
        static var steps: [TraceStep] {
            let messages = Fixtures.dmMessages
            func call(_ call: Int, _ result: Int) -> TracedCall {
                TracedCall(
                    call: messages[call].tool, id: messages[call].id, result: messages[result])
            }
            func said(_ id: String, _ text: String) -> TraceStep {
                .narration(
                    .with {
                        $0.id = id
                        $0.kind = .text
                        $0.content = text
                        $0.authorBotID = Fixtures.bot.id
                    })
            }
            return [
                said("trace-1", "I'll look at what is written on heat pumps in older houses."),
                .tools([call(3, 4), call(5, 6)]),
                said("trace-2", "That is enough to write the brief."),
                .tools([call(7, 8)]),
            ]
        }
    }

    /// A line showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            duration: state == .noTimes ? nil : 252, steps: PreviewState.steps,
            isExpanded: state == .open)
    }
}

#Preview("Folded") { RunTrace(previewState: .folded).padding() }
#Preview("Open") { RunTrace(previewState: .open).padding() }
