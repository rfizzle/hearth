// A deliverable's files are one chip: the spec with the page and PDF beside
// it, standing where the spec stood; files of another stem or folder, and
// pages with no spec, stay files.

import HearthKit
import Testing

@testable import HearthUI

private func file(_ path: String) -> Hearth_V1_FileRef {
    .with {
        $0.path = path
        $0.name = String(path.split(separator: "/").last ?? "")
    }
}

@Test func theThreeFilesOfOneStemAreOneChip() {
    let items = DeliverableGroup.items([
        file("/b/Dash.html"), file("/b/Dash.deliverable.json"), file("/b/Dash.pdf"),
        file("/b/notes.md"),
    ])
    #expect(items.map(\.id) == ["/b/Dash.deliverable.json", "/b/notes.md"])
    guard case .deliverable(let group) = items[0] else {
        Issue.record("the first item is not a deliverable")
        return
    }
    #expect(group.stem == "Dash")
    #expect(group.html?.path == "/b/Dash.html")
    #expect(group.pdf?.path == "/b/Dash.pdf")
}

@Test func aLoneSpecIsTheSameChip() {
    let items = DeliverableGroup.items([file("/b/Dash.deliverable.json")])
    guard case .deliverable(let group) = items.first else {
        Issue.record("a lone spec is not a deliverable")
        return
    }
    #expect(items.count == 1 && group.html == nil && group.pdf == nil)
}

@Test func aPageWithNoSpecOrAnotherFolderStaysAFile() {
    let items = DeliverableGroup.items([
        file("/b/Dash.html"), file("/c/Dash.deliverable.json"), file("/b/Dash.pdf"),
    ])
    #expect(items.map(\.id) == ["/b/Dash.html", "/c/Dash.deliverable.json", "/b/Dash.pdf"])
    #expect(items[0] == .file(file("/b/Dash.html")))
}
