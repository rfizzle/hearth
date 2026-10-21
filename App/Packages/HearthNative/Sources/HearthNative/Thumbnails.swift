// Thumbnails makes the small picture a file chip shows: Quick Look's own
// generator renders it out of process, so the app never reads the file's
// content. A picture is cached per path, modification date and size, and so is
// a miss; an edited file has a new date and is asked for again. The app target
// hands it to the views as the fileThumbnail environment action.
// See docs/design.md#the-transcript

import CoreGraphics
import Foundation
import QuickLookThumbnailing

/// The thumbnails of files, as the app reaches Quick Look.
public actor Thumbnails {
    /// The one cache the app shares.
    public static let shared = Thumbnails()

    private static let capacity = 256
    private var cache: [String: CGImage?] = [:]

    public init() {}

    /// The picture for the file at the absolute `path`, `side` points square at
    /// twice the density, or nil when there is none: the file is gone or Quick
    /// Look has no picture of its type.
    public func thumbnail(path: String, side: CGFloat) async -> CGImage? {
        let url = URL(fileURLWithPath: path)
        guard
            let modified = try? url.resourceValues(forKeys: [.contentModificationDateKey])
                .contentModificationDate
        else { return nil }
        let key = "\(path)\n\(modified.timeIntervalSince1970)\n\(side)"
        if let hit = cache[key] { return hit }

        let request = QLThumbnailGenerator.Request(
            fileAt: url, size: CGSize(width: side, height: side), scale: 2,
            representationTypes: .thumbnail)
        let image = try? await QLThumbnailGenerator.shared.generateBestRepresentation(for: request)
            .cgImage
        if cache.count >= Self.capacity { cache.removeAll() }
        cache[key] = .some(image)
        return image
    }
}
