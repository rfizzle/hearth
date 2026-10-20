// PageRow is one handbook page: its title, a pill for who reads it (everyone
// or one bot by name) and how much room it takes. Clicking it opens the page;
// the context menu deletes, asking first. The row decides nothing; each action
// calls back.
// See docs/model.md#knowledge

import HearthDesign
import HearthKit
import SwiftUI

/// A handbook page in the inspector.
public struct PageRow: View {
    let title: String
    let scope: String
    let bytes: Int64
    let onOpen: () -> Void
    let onDelete: () -> Void

    @State private var isConfirming = false

    public init(
        title: String, scope: String, bytes: Int64,
        onOpen: @escaping () -> Void, onDelete: @escaping () -> Void
    ) {
        self.title = title
        self.scope = scope
        self.bytes = bytes
        self.onOpen = onOpen
        self.onDelete = onDelete
    }

    public var body: some View {
        Button(action: onOpen) {
            VStack(alignment: .leading, spacing: Theme.Spacing.xs) {
                Text(title)
                    .font(Theme.body)
                    .lineLimit(2)
                HStack(spacing: Theme.Spacing.sm) {
                    Pill(scope)
                    Text(bytes.roomText)
                        .font(Theme.caption)
                        .foregroundStyle(Theme.color(.textSecondary))
                        .monospacedDigit()
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.vertical, Theme.Spacing.xs)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .contextMenu {
            Button("Open", systemImage: "doc.text", action: onOpen)
            Divider()
            Button("Delete…", systemImage: "trash", role: .destructive) { isConfirming = true }
        }
        .confirmationDialog(
            "Remove this page?", isPresented: $isConfirming, titleVisibility: .visible
        ) {
            Button("Delete", role: .destructive, action: onDelete)
        } message: {
            Text("\(title) stops going into every run.")
        }
        .accessibilityElement(children: .combine)
        .accessibilityValue("\(scope), \(bytes.roomText)")
    }
}

extension PageRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case everyone
        case oneBot
        case long

        var page: Hearth_V1_HandbookPage {
            switch self {
            case .oneBot: Fixtures.handbookPages[1]
            default: Fixtures.handbookPages[0]
            }
        }

        var title: String {
            self == .long
                ? "House style for every brief, summary and message the team sends the owner"
                : page.title
        }

        var scope: String { self == .oneBot ? Fixtures.bot.name : "Everyone" }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(
            title: state.title, scope: state.scope, bytes: state.page.bytes,
            onOpen: {}, onDelete: {})
    }
}

#Preview("Everyone") { PageRow(previewState: .everyone).padding() }
#Preview("One bot") { PageRow(previewState: .oneBot).padding() }
