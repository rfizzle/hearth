// FileIcon is the icon the Mac shows for a file of a given name: the Finder's
// own markdown, PDF or image icon. It wraps an Image over
// NSWorkspace.shared.icon(for:), the one AppKit call in HearthDesign besides
// NSColor, because SwiftUI has no API that returns a document icon for a type.
// The icon is decoration beside the file's name, so it is hidden from
// VoiceOver, and it scales with Dynamic Type.

import AppKit
import SwiftUI
import UniformTypeIdentifiers

/// The system's icon for a file name.
public struct FileIcon: View {
    let filename: String

    @ScaledMetric(relativeTo: .body) private var side: CGFloat = Theme.Size.fileIcon

    public init(filename: String) {
        self.filename = filename
    }

    public var body: some View {
        Image(nsImage: NSWorkspace.shared.icon(for: Self.contentType(of: filename)))
            .resizable()
            .scaledToFit()
            .frame(width: side, height: side)
            .accessibilityHidden(true)
    }

    /// The type named by `filename`'s extension, or plain data when it names none.
    static func contentType(of filename: String) -> UTType {
        UTType(filenameExtension: (filename as NSString).pathExtension) ?? .data
    }
}

extension FileIcon {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case markdown
        case pdf
        case image
        case unknown

        /// The file name for this state.
        public var filename: String {
            switch self {
            case .markdown: "notes.md"
            case .pdf: "report.pdf"
            case .image: "chart.png"
            case .unknown: "README"
            }
        }
    }

    /// An icon showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(filename: state.filename)
    }
}

#Preview("Markdown") { FileIcon(previewState: .markdown).padding() }
#Preview("PDF") { FileIcon(previewState: .pdf).padding() }
#Preview("Image") { FileIcon(previewState: .image).padding() }
#Preview("Unknown") { FileIcon(previewState: .unknown).padding() }
