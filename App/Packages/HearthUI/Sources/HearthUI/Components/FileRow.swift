// FileRow is a file a bot wrote, drawn as a bordered chip with the Finder's
// icon for its type; the chip is the action and clicking it calls the
// revealInFinder environment action with the file's path. The context menu
// offers the same. It is a system Button, so keyboard and VoiceOver behaviour
// is the system's; its label names the file and the action, so a list of
// several reads apart.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// A file in the transcript.
public struct FileRow: View {
    let file: Hearth_V1_FileRef

    @Environment(\.revealInFinder) private var revealInFinder

    public init(file: Hearth_V1_FileRef) {
        self.file = file
    }

    public var body: some View {
        Button {
            revealInFinder(file.path)
        } label: {
            HStack(spacing: Theme.Spacing.xs) {
                FileIcon(filename: file.name)
                Text(file.name)
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textPrimary))
                    .lineLimit(1)
                    .truncationMode(.middle)
            }
            .padding(.horizontal, Theme.Spacing.sm)
            .padding(.vertical, Theme.Spacing.xs)
            .overlay(
                RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                    .stroke(.separator)
            )
            .contentShape(RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous))
        }
        .buttonStyle(.plain)
        .help("Show in Finder")
        .accessibilityLabel("\(file.name), show in Finder")
        .contextMenu {
            Button("Show in Finder") { revealInFinder(file.path) }
        }
    }
}

extension FileRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case brief
        case longName

        var file: Hearth_V1_FileRef {
            switch self {
            case .brief:
                Fixtures.briefFile
            case .longName:
                .with {
                    $0.name = "Heat pump grants and installers within twenty miles, 2026.md"
                    $0.path = Fixtures.bot.folderPath + "/" + $0.name
                }
            }
        }
    }

    /// A row showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(file: state.file)
    }
}

#Preview("Brief") { FileRow(previewState: .brief).padding() }
#Preview("Long name") { FileRow(previewState: .longName).padding() }
