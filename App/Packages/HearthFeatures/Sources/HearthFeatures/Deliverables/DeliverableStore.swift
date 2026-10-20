// DeliverableStore holds one shaped deliverable as its file says: it reads the
// spec file at a path, decodes it strictly and keeps either the spec or the
// fact that this Hearth cannot show the file, and notes whether the page and
// the PDF the core wrote beside it are there. The window reloads it whenever it
// becomes key, so a bot that rewrote the file shows its new words. The store
// decides nothing: what the file says is what is shown.
// See docs/decisions.md (D45)

import Foundation
import HearthKit
import Observation

/// The store behind a deliverable's window.
@MainActor
@Observable
public final class DeliverableStore {
    /// What the window has to show.
    public enum State: Equatable, Sendable {
        /// The file has not been read yet.
        case loading
        /// The file decoded.
        case shown(Deliverable)
        /// The file is gone, too large, or not a deliverable.
        case unreadable
    }

    /// The largest spec file read; a bigger one is not a deliverable.
    nonisolated static let sizeCap = 4 << 20

    /// The suffix of a spec file, whose stem names the page and the PDF.
    nonisolated static let specSuffix = ".deliverable.json"

    /// The spec file's absolute path.
    public let path: String
    /// The file as last read.
    public private(set) var state: State = .loading
    /// Whether the page beside the spec is there.
    public private(set) var hasPage = false
    /// Whether the PDF beside the spec is there.
    public private(set) var hasPDF = false

    /// Whether the state was given rather than read, as in the gallery.
    private let isFixed: Bool

    /// A store for the spec file at `path`.
    public init(path: String) {
        self.path = path
        isFixed = false
    }

    /// A store already showing `spec`, for the gallery and previews.
    public init(path: String, previewing spec: Deliverable, hasPage: Bool, hasPDF: Bool) {
        self.path = path
        isFixed = true
        state = .shown(spec)
        self.hasPage = hasPage
        self.hasPDF = hasPDF
    }

    /// The page beside the spec file.
    public var pagePath: String { sibling("html") }

    /// The PDF beside the spec file.
    public var pdfPath: String { sibling("pdf") }

    /// The window's title: the deliverable's own, else the file's stem.
    public var title: String {
        if case .shown(let spec) = state, !spec.title.isEmpty { return spec.title }
        return stem
    }

    /// The file's name without the suffix.
    var stem: String {
        let name = (path as NSString).lastPathComponent
        return name.hasSuffix(Self.specSuffix) ? String(name.dropLast(Self.specSuffix.count)) : name
    }

    /// Reads the file again, replacing what is shown.
    public func load() async {
        guard !isFixed else { return }
        let path = path
        let read = await Task.detached { Self.read(path) }.value
        state = read.spec.map(State.shown) ?? .unreadable
        hasPage = read.hasPage
        hasPDF = read.hasPDF
    }

    private func sibling(_ ext: String) -> String {
        path.hasSuffix(Self.specSuffix)
            ? String(path.dropLast(Self.specSuffix.count)) + "." + ext : path + "." + ext
    }

    /// What one read of the file and its neighbours found.
    private struct Read: Sendable {
        var spec: Deliverable?
        var hasPage = false
        var hasPDF = false
    }

    private nonisolated static func read(_ path: String) -> Read {
        let fileManager = FileManager.default
        let base =
            path.hasSuffix(specSuffix) ? String(path.dropLast(specSuffix.count)) : path
        var read = Read(
            hasPage: fileManager.fileExists(atPath: base + ".html"),
            hasPDF: fileManager.fileExists(atPath: base + ".pdf"))
        guard
            let size = (try? fileManager.attributesOfItem(atPath: path))?[.size] as? Int,
            size <= sizeCap,
            let data = fileManager.contents(atPath: path)
        else { return read }
        read.spec = try? Deliverable(json: data)
        return read
    }
}
