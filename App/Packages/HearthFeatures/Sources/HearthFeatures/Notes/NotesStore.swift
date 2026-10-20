// NotesStore holds what one bot remembers as the core last listed it: the
// pinned notes first, then the rest, and how many the bot may keep. It fetches
// the list, applies each note change the relay delivers (a new note joins, an
// edited one is replaced, a deleted one leaves) and refetches on a resync. Pin,
// edit and delete are calls; the core's answer shows at once and its change
// event, arriving after, finds the note already as it says. The store decides
// nothing about which notes the bot keeps.
// See docs/model.md#knowledge

import Foundation
import HearthKit
import Observation

/// The store behind the inspector's Notes section.
@MainActor
@Observable
public final class NotesStore {
    /// The bot these notes belong to.
    public let botID: String
    /// What the bot remembers, the pinned first.
    public private(set) var notes: [Hearth_V1_Note] = []
    /// How many notes the bot may keep.
    public private(set) var cap = 0
    /// Whether the first list has come back.
    public private(set) var loaded = false
    /// The last failed call's text, if one failed.
    public private(set) var error: String?

    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store for `botID` over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay, botID: String) {
        self.client = client
        self.relay = relay
        self.botID = botID
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// The section header's count, "3 of 40".
    public var countText: String { "\(notes.count) of \(cap)" }

    /// Fetches the bot's notes, replacing whatever is shown.
    public func load() async {
        do {
            let response = try await client.listNotes(botID: botID)
            notes = Self.pinnedFirst(response.notes)
            cap = Int(response.cap)
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Pins a note so the bot never tidies it away, or lets it go.
    public func pin(_ note: Hearth_V1_Note, _ pinned: Bool) async {
        await run { apply(try await client.pinNote(id: note.id, pinned: pinned)) }
    }

    /// Replaces the words of a note.
    public func edit(_ note: Hearth_V1_Note, text: String) async {
        await run { apply(try await client.editNote(id: note.id, text: text)) }
    }

    /// Makes the bot forget a note.
    public func delete(_ note: Hearth_V1_Note) async {
        await run {
            try await client.deleteNote(id: note.id)
            remove(note.id)
        }
    }

    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .noteChanged(let change) where change.botID == botID:
            if change.deleted {
                remove(change.note.id)
            } else {
                apply(change.note)
            }
        case .resync:
            await load()
        default:
            break
        }
    }

    private func run(_ call: () async throws -> Void) async {
        do {
            try await call()
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Replaces a note in place, or adds a new one ahead of the unpinned.
    private func apply(_ note: Hearth_V1_Note) {
        var next = notes
        if let index = next.firstIndex(where: { $0.id == note.id }) {
            next[index] = note
        } else {
            next.insert(note, at: next.firstIndex { !$0.pinned } ?? next.endIndex)
        }
        notes = Self.pinnedFirst(next)
    }

    private func remove(_ id: String) {
        notes.removeAll { $0.id == id }
    }

    private static func pinnedFirst(_ notes: [Hearth_V1_Note]) -> [Hearth_V1_Note] {
        notes.filter(\.pinned) + notes.filter { !$0.pinned }
    }
}
