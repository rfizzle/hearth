// DeliverableGroup gathers the files of one shaped deliverable. The core writes
// `<stem>.deliverable.json` and, beside it, `<stem>.html` and `<stem>.pdf`; the
// transcript shows the three as one chip. A file that is not part of such a
// set stays a file, and the chip stands where its spec file stood.

import HearthKit

/// The files of one deliverable.
struct DeliverableGroup: Equatable {
    /// The suffix that marks the spec file.
    static let specSuffix = ".deliverable.json"

    let spec: Hearth_V1_FileRef
    let html: Hearth_V1_FileRef?
    let pdf: Hearth_V1_FileRef?

    /// The file's name without the suffix, the title when the spec cannot be read.
    var stem: String {
        String(spec.name.dropLast(Self.specSuffix.count))
    }

    /// What the transcript draws for one file or one deliverable.
    enum Item: Equatable {
        case file(Hearth_V1_FileRef)
        case deliverable(DeliverableGroup)

        var id: String {
            switch self {
            case .file(let file): file.path
            case .deliverable(let group): group.spec.path
            }
        }
    }

    /// `files` as items: each spec file with the page and PDF beside it
    /// (same folder, same stem) as one deliverable, the rest as they were.
    static func items(_ files: [Hearth_V1_FileRef]) -> [Item] {
        var taken = Set<String>()
        var groups: [String: DeliverableGroup] = [:]
        for spec in files where spec.path.hasSuffix(specSuffix) {
            let base = String(spec.path.dropLast(specSuffix.count))
            let html = files.first { $0.path == base + ".html" }
            let pdf = files.first { $0.path == base + ".pdf" }
            groups[spec.path] = DeliverableGroup(spec: spec, html: html, pdf: pdf)
            taken.formUnion([html?.path, pdf?.path].compactMap { $0 })
        }
        return files.compactMap { file in
            if let group = groups[file.path] { return .deliverable(group) }
            return taken.contains(file.path) ? nil : .file(file)
        }
    }
}
