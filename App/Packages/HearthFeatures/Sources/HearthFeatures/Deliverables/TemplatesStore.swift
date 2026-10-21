// TemplatesStore holds the deliverable templates as the core lists them: the
// five Hearth ships and the owner's own, each with the problem the core found
// in one that does not load. It lists, saves and deletes through the client and
// shows the core's refusal in its own words. There is no event for a template,
// so the Settings pane reloads it when its window becomes key. The store
// decides nothing: the core validates, refuses a shipped id and writes only
// into the Templates folder.
// See docs/model.md#work, docs/decisions.md (D43)

import Foundation
import HearthKit
import Observation

/// The store behind Settings › Deliverables and the Save as Template sheet.
@MainActor
@Observable
public final class TemplatesStore {
    /// The templates Hearth ships, in the core's order.
    public private(set) var shipped: [Hearth_V1_DeliverableTemplate] = []
    /// The owner's templates, a failing one included with its problem.
    public private(set) var owned: [Hearth_V1_DeliverableTemplate] = []
    /// Whether the list has been asked for.
    public private(set) var loaded = false
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// Whether a save is in flight.
    public private(set) var isSaving = false

    private let client: any HearthClient

    /// A store over `client`; `error` starts the store with a failure shown,
    /// for the gallery.
    public init(client: any HearthClient, error: String? = nil) {
        self.client = client
        self.error = error
    }

    /// The folder the owner's templates sit in, from the first one listed; nil
    /// while there is none.
    public var folderPath: String? {
        owned.first.map { ($0.path as NSString).deletingLastPathComponent }
    }

    /// Lists the templates again, replacing what is shown.
    public func load() async {
        do {
            let all = try await client.listDeliverableTemplates()
            shipped = all.filter(\.shipped)
            owned = all.filter { !$0.shipped }
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
        loaded = true
    }

    /// Sends `json` as a template; true when the core kept it. A refusal is
    /// kept in `error` and the list is left as it was.
    public func save(json: String) async -> Bool {
        isSaving = true
        defer { isSaving = false }
        do {
            _ = try await client.saveDeliverableTemplate(json: json)
            error = nil
        } catch {
            self.error = error.localizedDescription
            return false
        }
        await load()
        return true
    }

    /// Deletes the owner's template `id`.
    public func delete(id: String) async {
        do {
            try await client.deleteDeliverableTemplate(id: id)
            error = nil
        } catch {
            self.error = error.localizedDescription
            return
        }
        await load()
    }

    /// Forgets the last failure, as a sheet does when it opens.
    public func clearError() { error = nil }
}
