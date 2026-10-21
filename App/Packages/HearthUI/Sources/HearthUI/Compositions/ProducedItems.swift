// ProducedItems is the list behind the card of what a conversation produced:
// every file its messages carry, once by path with the newest copy winning,
// grouped as the transcript groups them (a deliverable's files are one item),
// deliverables first and then other files, newest first within each. It reads
// nothing but the messages, so the card never claims more than the thread
// shows.
// See docs/design.md#the-transcript

import HearthKit

/// What a conversation's messages produced, for the ProducedCard.
public struct ProducedList: Equatable {
    let items: [DeliverableGroup.Item]

    /// The list `messages` (ascending by seq) make.
    public init(messages: [Hearth_V1_Message]) {
        items = ProducedItems.build(messages: messages)
    }

    init(items: [DeliverableGroup.Item]) {
        self.items = items
    }

    /// Whether nothing was produced, so the card has nothing to say.
    public var isEmpty: Bool { items.isEmpty }
}

/// The rules for the list.
enum ProducedItems {
    /// How many items the card lists before "Show all".
    static let limit = 5

    /// The items of `messages` (ascending by seq): deliverables first, then
    /// other files, newest first within each, one per path.
    static func build(messages: [Hearth_V1_Message]) -> [DeliverableGroup.Item] {
        var seen = Set<String>()
        var files: [Hearth_V1_FileRef] = []
        for message in messages.reversed() {
            for file in message.files.reversed() where seen.insert(file.path).inserted {
                files.append(file)
            }
        }
        let items = DeliverableGroup.items(files)
        return items.filter(\.isDeliverable) + items.filter { !$0.isDeliverable }
    }

    /// The first `limit` of `items`, or all of them once `showAll`.
    static func visible(
        _ items: [DeliverableGroup.Item], showAll: Bool
    ) -> [DeliverableGroup.Item] {
        showAll ? items : Array(items.prefix(limit))
    }

    /// The collapsed pill's words.
    static func pill(count: Int) -> String {
        count == 1 ? "Produced 1 item" : "Produced \(count) items"
    }
}

extension DeliverableGroup.Item {
    var isDeliverable: Bool {
        if case .deliverable = self { return true }
        return false
    }
}
