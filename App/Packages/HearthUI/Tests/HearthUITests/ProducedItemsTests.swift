// The list the produced card shows, built from a conversation's messages.

import HearthKit
import Testing

@testable import HearthUI

private func ref(_ name: String) -> Hearth_V1_FileRef {
    .with {
        $0.name = name
        $0.path = "/Work/Harbor/" + name
    }
}

private func message(_ seq: Int64, _ names: [String]) -> Hearth_V1_Message {
    .with {
        $0.id = "m\(seq)"
        $0.seq = seq
        $0.files = names.map(ref)
    }
}

private func names(_ items: [DeliverableGroup.Item]) -> [String] {
    items.map {
        switch $0 {
        case .file(let file): file.name
        case .deliverable(let group): group.spec.name
        }
    }
}

@Test func producedListsNewestFirst() {
    let items = ProducedItems.build(messages: [
        message(1, ["a.md"]), message(2, ["b.md", "c.md"]),
    ])
    #expect(names(items) == ["c.md", "b.md", "a.md"])
}

@Test func producedKeepsOneItemPerPath() {
    let items = ProducedItems.build(messages: [
        message(1, ["a.md", "b.md"]), message(2, ["a.md"]),
    ])
    #expect(names(items) == ["a.md", "b.md"])
}

@Test func producedPutsDeliverablesFirstAsOneItem() {
    let items = ProducedItems.build(messages: [
        message(1, ["Plan.deliverable.json", "Plan.html", "Plan.pdf"]),
        message(2, ["notes.md"]),
    ])
    #expect(names(items) == ["Plan.deliverable.json", "notes.md"])
}

@Test func producedIsEmptyWithoutFiles() {
    #expect(ProducedList(messages: [message(1, [])]).isEmpty)
    #expect(ProducedList(messages: []).isEmpty)
}

@Test func producedShowsFiveThenAll() {
    let items = ProducedItems.build(messages: [message(1, (1...7).map { "f\($0).md" })])
    #expect(ProducedItems.visible(items, showAll: false).count == 5)
    #expect(ProducedItems.visible(items, showAll: true).count == 7)
    #expect(ProducedItems.pill(count: 1) == "Produced 1 item")
    #expect(ProducedItems.pill(count: 4) == "Produced 4 items")
}
