// FileRow names a file a bot wrote and offers Show in Finder, which calls the
// revealInFinder environment action with the file's path. The button is a
// system Button, so keyboard and VoiceOver behaviour is the system's; its
// label names the file, so a list of several reads apart.

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
        HStack {
            Image(systemName: "doc.text")
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
            Text(file.name)
                .font(Theme.body)
                .foregroundStyle(Theme.color(.textPrimary))
                .lineLimit(1)
                .truncationMode(.middle)
            Spacer()
            Button("Show in Finder") { revealInFinder(file.path) }
                .accessibilityLabel("Show \(file.name) in Finder")
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
