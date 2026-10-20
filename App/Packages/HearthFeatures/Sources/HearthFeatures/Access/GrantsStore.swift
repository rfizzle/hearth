// GrantsStore holds what one bot may do as the Access sheet edits it: a row per
// capability with its mode, and the folders the file capabilities reach. The
// core keeps one grant per capability and per folder; the sheet shows one set
// of modes for every folder, so the store folds the core's grants into rows
// and a folder list, and Save composes the grants again, one per file
// capability per folder at the row's mode. The bot's own folder stays; the
// owner adds and removes the others. Where the owner must be asked every
// time, "allowed" cannot be chosen. The core enforces all of it; the store
// only edits what is shown, and Cancel puts back what the core last said.
// See docs/model.md#access

import Foundation
import HearthKit
import Observation

/// The store behind the Access sheet.
@MainActor
@Observable
public final class GrantsStore {
    /// One capability and the mode it is granted at.
    public struct Row: Identifiable, Equatable, Sendable {
        public let capability: Hearth_V1_Capability
        /// The core's plain words for the capability.
        public let title: String
        /// Whether "allowed" may be chosen.
        public let canAllow: Bool
        public var mode: Hearth_V1_GrantMode

        public var id: Int { capability.rawValue }

        /// Whether the capability reaches folders.
        public var reachesFolders: Bool { capability.reachesFolders }
    }

    /// A folder the file capabilities reach.
    public struct Folder: Identifiable, Equatable, Sendable {
        public let path: String
        /// The bot's own folder, which stays.
        public let isOwn: Bool

        public var id: String { path }
    }

    /// The bot these grants belong to.
    public let botID: String
    /// One row per capability, in the core's order.
    public private(set) var rows: [Row] = []
    /// The folders, the bot's own first.
    public private(set) var folders: [Folder] = []
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// Whether a Save is waiting on the core.
    public private(set) var isSaving = false
    /// Whether the last Save went through, so the sheet can close.
    public private(set) var didSave = false

    private var kept: [Hearth_V1_Grant] = []
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

    /// Fetches the bot's grants, replacing whatever is shown.
    public func load() async {
        do {
            apply(try await client.listGrants(botID: botID))
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// The grants the rows and folders stand for: a grant per file capability
    /// per folder at the row's mode, and one for each other capability.
    public var grants: [Hearth_V1_Grant] {
        rows.flatMap { row in
            let make = { (folder: Folder?) in
                Hearth_V1_Grant.with {
                    $0.capability = row.capability
                    $0.mode = row.mode
                    $0.folder = folder?.path ?? ""
                    $0.title = row.title
                    $0.canAllow = row.canAllow
                    $0.ownFolder = folder?.isOwn ?? false
                }
            }
            return row.reachesFolders ? folders.map { make($0) } : [make(nil)]
        }
    }

    /// Sets a capability's mode. "Allowed" is refused where the owner must be
    /// asked every time.
    public func set(mode: Hearth_V1_GrantMode, for capability: Hearth_V1_Capability) {
        guard let index = rows.firstIndex(where: { $0.capability == capability }) else { return }
        guard mode != .allowed || rows[index].canAllow else { return }
        rows[index].mode = mode
    }

    /// The mode a capability is at, for a control's binding.
    public func mode(for capability: Hearth_V1_Capability) -> Hearth_V1_GrantMode {
        rows.first { $0.capability == capability }?.mode ?? .ask
    }

    /// Adds a folder the file capabilities will reach; one already listed is
    /// not added twice.
    public func addFolder(_ path: String) {
        let path = path.hasSuffix("/") && path.count > 1 ? String(path.dropLast()) : path
        guard !path.isEmpty, !folders.contains(where: { $0.path == path }) else { return }
        folders.append(Folder(path: path, isOwn: false))
    }

    /// Removes an added folder; the bot's own stays.
    public func removeFolder(_ path: String) {
        folders.removeAll { $0.path == path && !$0.isOwn }
    }

    /// Sends the rows as shown and shows what the core kept.
    public func save() async {
        isSaving = true
        defer { isSaving = false }
        do {
            apply(try await client.setGrants(botID: botID, grants))
            error = nil
            didSave = true
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Puts back what the core last said, dropping the owner's edits.
    public func discard() {
        apply(kept)
        error = nil
    }

    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .grantsChanged(let change) where change.botID == botID:
            apply(change.grants)
        case .resync:
            await load()
        default:
            break
        }
    }

    private func apply(_ grants: [Hearth_V1_Grant]) {
        kept = grants
        var rows: [Row] = []
        var folders: [Folder] = []
        for grant in grants {
            if !rows.contains(where: { $0.capability == grant.capability }) {
                rows.append(
                    Row(
                        capability: grant.capability, title: grant.title,
                        canAllow: grant.canAllow, mode: grant.mode))
            }
            if grant.capability.reachesFolders, !grant.folder.isEmpty {
                if let index = folders.firstIndex(where: { $0.path == grant.folder }) {
                    if grant.ownFolder { folders[index] = Folder(path: grant.folder, isOwn: true) }
                } else {
                    folders.append(Folder(path: grant.folder, isOwn: grant.ownFolder))
                }
            }
        }
        self.rows = rows
        self.folders = folders
    }
}

extension Hearth_V1_Capability {
    /// The three file capabilities, which reach folders.
    var reachesFolders: Bool {
        switch self {
        case .filesRead, .filesWrite, .filesDelete: true
        default: false
        }
    }
}
