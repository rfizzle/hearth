// AccessRow is one thing a bot may do: its symbol, its name, a line saying what
// it lets the bot do, and a segmented control for Allowed, Ask me or Not
// granted. The control is a system Picker, so keyboard and VoiceOver behaviour
// is the system's. Where the owner must be asked every time the Allowed
// segment is absent, and a tooltip on the control says why. The row edits a
// binding and decides nothing.
// See docs/model.md#access

import HearthDesign
import HearthKit
import SwiftUI

/// One capability and the mode it is granted at.
public struct AccessRow: View {
    let symbol: String
    let title: String
    let summary: String
    let canAllow: Bool
    @Binding var mode: Hearth_V1_GrantMode

    public init(
        symbol: String, title: String, summary: String, canAllow: Bool,
        mode: Binding<Hearth_V1_GrantMode>
    ) {
        self.symbol = symbol
        self.title = title
        self.summary = summary
        self.canAllow = canAllow
        _mode = mode
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.md) {
            Image(systemName: symbol)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textSecondary))
                .frame(width: Theme.Size.avatar)
                .accessibilityHidden(true)
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(title).font(Theme.body)
                Text(summary)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .fixedSize(horizontal: false, vertical: true)
            }
            Spacer(minLength: Theme.Spacing.md)
            Picker(title, selection: $mode) {
                ForEach(Self.choices(canAllow: canAllow), id: \.self) { choice in
                    Text(Self.name(choice)).tag(choice)
                }
            }
            .pickerStyle(.segmented)
            .labelsHidden()
            .frame(width: Self.controlWidth)
            .help(canAllow ? "" : "hearth always asks before deleting")
        }
        .padding(.vertical, Theme.Spacing.sm)
    }

    /// About 240 points: three short words in a segmented control.
    static let controlWidth: CGFloat = 240

    /// The segments a row offers, in order.
    static func choices(canAllow: Bool) -> [Hearth_V1_GrantMode] {
        canAllow ? [.allowed, .ask, .notGranted] : [.ask, .notGranted]
    }

    static func name(_ mode: Hearth_V1_GrantMode) -> String {
        switch mode {
        case .allowed: "Allowed"
        case .notGranted: "Not granted"
        default: "Ask me"
        }
    }
}

extension AccessRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case allowed
        case ask
        case notGranted
        case cannotAllow

        var mode: Hearth_V1_GrantMode {
            switch self {
            case .allowed: .allowed
            case .ask, .cannotAllow: .ask
            case .notGranted: .notGranted
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        if state == .cannotAllow {
            self.init(
                symbol: "trash", title: "Delete files",
                summary: "Move files in the folders below to the Trash.", canAllow: false,
                mode: .constant(state.mode))
        } else {
            self.init(
                symbol: "doc.text", title: "Read files",
                summary: "Open and read files in the folders below.", canAllow: true,
                mode: .constant(state.mode))
        }
    }
}

#Preview("Allowed") { AccessRow(previewState: .allowed).padding() }
#Preview("Cannot allow") { AccessRow(previewState: .cannotAllow).padding() }
