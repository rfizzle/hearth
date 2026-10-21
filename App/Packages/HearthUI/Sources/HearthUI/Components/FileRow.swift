// FileRow is a file in the transcript, a bot's or one the owner attached,
// drawn as a bordered chip with the Finder's icon for its type. Clicking it, or
// Return on it when focused, calls the revealInFinder environment action with
// the file's path; Space opens Quick Look on it, and the context menu offers
// both. It shows the file's thumbnail from the fileThumbnail action, and the
// type's icon without one. It is a focusable view with the button trait rather than a Button,
// since a Button takes Space for itself. Its label names the file and the
// action, so a list of several reads apart; the app only names the path, and
// Quick Look is the system's.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import QuickLook
import SwiftUI

/// A file in the transcript.
public struct FileRow: View {
    let file: Hearth_V1_FileRef

    @Environment(\.revealInFinder) private var revealInFinder
    @State private var previewURL: URL?

    public init(file: Hearth_V1_FileRef) {
        self.file = file
    }

    public var body: some View {
        HStack(spacing: Theme.Spacing.xs) {
            FileThumbnail(file: file)
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
        .onTapGesture { revealInFinder(file.path) }
        .focusable()
        .onKeyPress(.return) {
            revealInFinder(file.path)
            return .handled
        }
        .onKeyPress(.space) {
            previewURL = URL(fileURLWithPath: file.path)
            return .handled
        }
        .quickLookPreview($previewURL)
        .help("Quick Look (Space)")
        .accessibilityElement(children: .ignore)
        .accessibilityAddTraits(.isButton)
        .accessibilityLabel("\(file.name), show in Finder")
        .accessibilityAction { revealInFinder(file.path) }
        .accessibilityAction(named: "Quick Look") {
            previewURL = URL(fileURLWithPath: file.path)
        }
        .contextMenu {
            Button("Quick Look") { previewURL = URL(fileURLWithPath: file.path) }
            Button("Show in Finder") { revealInFinder(file.path) }
        }
    }
}

extension FileRow {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case brief
        case longName
        /// A PDF whose thumbnail the gallery supplies.
        case thumbnail

        var file: Hearth_V1_FileRef {
            switch self {
            case .brief:
                Fixtures.briefFile
            case .thumbnail:
                .with {
                    $0.name = "Q3 sales figures.pdf"
                    $0.path = Fixtures.bot.folderPath + "/" + $0.name
                }
            case .longName:
                .with {
                    $0.name = "Competitor pricing and plans for teams of five to fifty, 2026.md"
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
#Preview("Thumbnail") { FileRow(previewState: .thumbnail).padding() }
#Preview("Long name") { FileRow(previewState: .longName).padding() }
