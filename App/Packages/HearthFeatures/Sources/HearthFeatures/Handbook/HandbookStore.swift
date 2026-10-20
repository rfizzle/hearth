// HandbookStore holds the handbook pages one bot reads as the core last listed
// them, the pages for everyone and the pages for this bot alone, and the meter
// of how much of the shared room they take. It fetches the list, applies each
// page change the relay delivers (a saved page joins or is replaced, a deleted
// one leaves, a page now meant for another bot leaves too) and refetches on a
// resync. Put and delete are calls; the core's answer shows at once and its
// change event, arriving after, finds the page already as it says. The store
// decides nothing about what a page may hold: the core refuses an empty title
// or a page over the cap in plain words, and the store keeps those words for
// the editor to show.
// See docs/model.md#knowledge

import Foundation
import HearthKit
import Observation

/// The store behind the inspector's Handbook section.
@MainActor
@Observable
public final class HandbookStore {
    /// The bot these pages are read for.
    public let botID: String
    /// The bot's name, for the scope chip.
    public let botName: String
    /// The pages this bot reads, in the core's order.
    public private(set) var pages: [Hearth_V1_HandbookPage] = []
    /// How much of the shared room the pages take.
    public private(set) var meter = Hearth_V1_ContextMeter()
    /// Whether the first list has come back.
    public private(set) var loaded = false
    /// The last failed list or delete's text, if one failed.
    public private(set) var error: String?
    /// The core's refusal of the last save, shown under the editor.
    public private(set) var saveError: String?
    /// Whether a save is waiting on the core.
    public private(set) var isSaving = false

    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store for `botID` over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay, botID: String, botName: String) {
        self.client = client
        self.relay = relay
        self.botID = botID
        self.botName = botName
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// The words on a page's scope pill: "Everyone", or the bot's name.
    public func scopeText(_ page: Hearth_V1_HandbookPage) -> String {
        page.scope.everyone ? "Everyone" : botName
    }

    /// Fetches the bot's pages and the meter, replacing whatever is shown.
    public func load() async {
        do {
            let response = try await client.listPages(botID: botID)
            pages = response.pages
            meter = response.meter
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Adds a page, or replaces the page `id` names. Returns whether the core
    /// accepted it; if not, `saveError` says why in the core's words.
    @discardableResult
    public func put(
        id: String = "", title: String, markdown: String, scope: Hearth_V1_Scope
    ) async -> Bool {
        isSaving = true
        defer { isSaving = false }
        do {
            let response = try await client.putPage(
                .with {
                    $0.id = id
                    $0.title = title
                    $0.markdown = markdown
                    $0.scope = scope
                })
            apply(response.page)
            meter = response.meter
            saveError = nil
            return true
        } catch {
            saveError = error.localizedDescription
            return false
        }
    }

    /// Removes a page from every run.
    public func delete(_ page: Hearth_V1_HandbookPage) async {
        do {
            meter = try await client.deletePage(id: page.id)
            pages.removeAll { $0.id == page.id }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Forgets a refusal, as a fresh editor starts without one.
    public func clearSaveError() {
        saveError = nil
    }

    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .handbookChanged(let change):
            if change.deleted {
                remove(change.page.id)
            } else {
                apply(change.page)
            }
        case .resync:
            await load()
        default:
            break
        }
    }

    /// Replaces a page in place or adds it last, when this bot reads it; a
    /// page changed to another bot's leaves. The meter follows the bytes.
    private func apply(_ page: Hearth_V1_HandbookPage) {
        guard page.scope.everyone || page.scope.botID == botID else {
            remove(page.id)
            return
        }
        if let index = pages.firstIndex(where: { $0.id == page.id }) {
            meter.usedBytes += page.bytes - pages[index].bytes
            pages[index] = page
        } else {
            meter.usedBytes += page.bytes
            pages.append(page)
        }
    }

    private func remove(_ id: String) {
        guard let index = pages.firstIndex(where: { $0.id == id }) else { return }
        meter.usedBytes -= pages[index].bytes
        pages.remove(at: index)
    }
}
