// AttachmentChip is a file the owner has dropped on the composer and not yet
// sent: the Finder's icon, the name and a small button that takes it off
// again. It is focusable with the system's focus ring; Space opens Quick Look
// on the file, as does the context menu, and Delete removes it. The app only
// names the path; Quick Look is the system's, so nothing is read here.
// The remove button is a system Button, and its label names the file.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import QuickLook
import SwiftUI

/// A file waiting to be sent with the next message.
public struct AttachmentChip: View {
    let file: Hearth_V1_FileRef
    let onRemove: () -> Void

    @State private var previewURL: URL?

    public init(file: Hearth_V1_FileRef, onRemove: @escaping () -> Void) {
        self.file = file
        self.onRemove = onRemove
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.xs) {
            FileThumbnail(file: file)
            Text(file.name)
                .font(Theme.callout)
                .foregroundStyle(Theme.color(.textPrimary))
                .lineLimit(1)
                .truncationMode(.middle)
            Button("Remove \(file.name)", systemImage: "xmark.circle.fill", action: onRemove)
                .labelStyle(.iconOnly)
                .buttonStyle(.plain)
                .foregroundStyle(Theme.color(.textSecondary))
                .help("Remove")
        }
        .padding(.horizontal, Theme.Spacing.sm)
        .padding(.vertical, Theme.Spacing.xs)
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                .stroke(.separator)
        )
        .contentShape(RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous))
        .focusable()
        .onKeyPress(.space) {
            previewURL = URL(fileURLWithPath: file.path)
            return .handled
        }
        .onKeyPress(.delete) {
            onRemove()
            return .handled
        }
        .quickLookPreview($previewURL)
        .help("Quick Look (Space)")
        .accessibilityElement(children: .contain)
        .accessibilityLabel("\(file.name), attached")
        .accessibilityAction(named: "Quick Look") {
            previewURL = URL(fileURLWithPath: file.path)
        }
        .contextMenu {
            Button("Quick Look") { previewURL = URL(fileURLWithPath: file.path) }
            Button("Remove") { onRemove() }
        }
    }
}

extension AttachmentChip {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case brief
        case longName

        var file: Hearth_V1_FileRef {
            switch self {
            case .brief: Fixtures.briefFile
            case .longName:
                .with {
                    $0.name = "Competitor pricing and plans for teams of five to fifty, 2026.md"
                    $0.path = Fixtures.bot.folderPath + "/" + $0.name
                }
            }
        }
    }

    /// A chip showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(file: state.file, onRemove: {})
    }
}

#Preview("Brief") { AttachmentChip(previewState: .brief).padding() }
#Preview("Long name") { AttachmentChip(previewState: .longName).padding() }
