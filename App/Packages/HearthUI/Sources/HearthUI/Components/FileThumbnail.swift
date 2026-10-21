// FileThumbnail is what a file chip shows beside the name: a small picture of
// the file's first page or the picture itself, from the fileThumbnail
// environment action, and the Finder's icon for its type (FileIcon) while that
// loads and when there is none. The app target sets the action from
// HearthNative, which asks Quick Look's generator; it runs out of process, so
// nothing here reads the file. The default action has none, which is what the
// gallery and previews want. The picture is decoration beside the name, so it
// is hidden from VoiceOver.
// See docs/design.md#the-transcript

import HearthDesign
import HearthKit
import SwiftUI

/// Makes a thumbnail of the file at a path.
public struct FileThumbnailAction: Sendable {
    let make: @Sendable (_ path: String, _ side: CGFloat) async -> CGImage?

    /// An action that returns a picture `side` points square for the file at
    /// an absolute path, or nil when there is none.
    public init(_ make: @escaping @Sendable (_ path: String, _ side: CGFloat) async -> CGImage?) {
        self.make = make
    }

    /// The thumbnail for `path`, or nil.
    public func callAsFunction(_ path: String, side: CGFloat) async -> CGImage? {
        await make(path, side)
    }
}

extension EnvironmentValues {
    /// Makes a file's thumbnail; set by the app target.
    @Entry public var fileThumbnail = FileThumbnailAction { _, _ in nil }
}

/// A file's thumbnail, or its type's icon.
struct FileThumbnail: View {
    let file: Hearth_V1_FileRef

    @Environment(\.fileThumbnail) private var fileThumbnail
    @ScaledMetric(relativeTo: .body) private var side: CGFloat = 28
    @State private var image: CGImage?

    var body: some View {
        Group {
            if let image {
                Image(decorative: image, scale: 2)
                    .resizable()
                    .scaledToFit()
                    .clipShape(
                        RoundedRectangle(cornerRadius: Theme.Radius.sm / 2, style: .continuous)
                    )
                    .overlay(
                        RoundedRectangle(cornerRadius: Theme.Radius.sm / 2, style: .continuous)
                            .stroke(.separator)
                    )
            } else {
                FileIcon(filename: file.name)
            }
        }
        .frame(width: side, height: side)
        .accessibilityHidden(true)
        .task(id: file.path) {
            image = await fileThumbnail(file.path, side: side)
        }
    }
}
