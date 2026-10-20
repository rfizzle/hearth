// FolderRow is a folder a bot may reach: the Finder's folder icon, the path
// truncated in the middle so both its start and its end stay readable, and
// either an "Own folder" pill for the folder hearth made for the bot, which
// stays, or a Remove button. The row decides nothing; Remove calls back.
// See docs/model.md#access

import AppKit
import HearthDesign
import HearthKit
import SwiftUI
import UniformTypeIdentifiers

/// A folder in the Access sheet.
public struct FolderRow: View {
    let path: String
    let isOwn: Bool
    let onRemove: () -> Void

    @ScaledMetric(relativeTo: .body) private var iconSide: CGFloat = Theme.Size.fileIcon

    public init(path: String, isOwn: Bool, onRemove: @escaping () -> Void) {
        self.path = path
        self.isOwn = isOwn
        self.onRemove = onRemove
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.sm) {
            Image(nsImage: NSWorkspace.shared.icon(for: .folder))
                .resizable()
                .scaledToFit()
                .frame(width: iconSide, height: iconSide)
                .accessibilityHidden(true)
            Text(path)
                .font(Theme.callout)
                .lineLimit(1)
                .truncationMode(.middle)
                .help(path)
            Spacer(minLength: Theme.Spacing.md)
            if isOwn {
                Pill("Own folder")
            } else {
                Button("Remove", action: onRemove)
                    .accessibilityLabel("Remove \(path)")
            }
        }
        .padding(.vertical, Theme.Spacing.xs)
    }
}

extension FolderRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case own
        case added
        case longPath

        var path: String {
            switch self {
            case .own: Fixtures.bot.folderPath
            case .added: "/Users/sam/Documents/Heat pumps"
            case .longPath:
                "/Users/sam/Library/Mobile Documents/com~apple~CloudDocs/Projects/2026/Home energy/Research"
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(path: state.path, isOwn: state == .own, onRemove: {})
    }
}

#Preview("Own") { FolderRow(previewState: .own).padding() }
#Preview("Added") { FolderRow(previewState: .added).padding() }
