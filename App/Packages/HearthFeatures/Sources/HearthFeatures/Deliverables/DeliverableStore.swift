// DeliverableStore holds one shaped deliverable as its file says: it reads the
// spec file at a path, decodes it strictly and keeps either the spec or the
// fact that this Hearth cannot show the file, and notes whether the page and
// the PDF the core wrote beside it are there. The window reloads it whenever it
// becomes key, so a bot that rewrote the file shows its new words. The store
// decides nothing: what the file says is what is shown.
//
// A widget bound to a measure is live when the store has a client: it reads
// the bound ids from the core after each load, again half a second after a
// project, usage, Inbox, note or handbook event (the core publishes no measure
// event, so those are the moments a figure can have moved), and at once on a
// resync. Reads go at most 100 ids at a time, the core's cap. The core says
// which values exist and what they are; the store only keeps them by id. A
// spec with no bound widget never asks.
// See docs/decisions.md (D45, D46)

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

    /// The core's last read of the bound measures, by id; empty before a read
    /// and when nothing is bound.
    public private(set) var values: [String: Hearth_V1_MeasureValue] = [:]

    /// The wait after an event before the measures are read again.
    var debounce: Duration = .milliseconds(500)

    /// The most ids one read may carry; the core's cap.
    nonisolated static let readCap = 100

    /// Whether the state was given rather than read, as in the gallery.
    private let isFixed: Bool
    private let client: (any HearthClient)?
    @ObservationIgnored private let relay: EventRelay?
    @ObservationIgnored private var registration: EventRelay.Registration?
    @ObservationIgnored private var pending: Task<Void, Never>?

    /// A store for the spec file at `path`; with a `client` its bound widgets
    /// are read, and with a `relay` they follow the core's events.
    public init(path: String, client: (any HearthClient)? = nil, relay: EventRelay? = nil) {
        self.path = path
        self.client = client
        self.relay = relay
        isFixed = false
        registration = relay?.register { [weak self] event in await self?.handle(event) }
    }

    /// A store already showing `spec` and `values`, for the gallery and
    /// previews.
    public init(
        path: String, previewing spec: Deliverable, hasPage: Bool, hasPDF: Bool,
        values: [String: Hearth_V1_MeasureValue] = [:]
    ) {
        self.path = path
        client = nil
        relay = nil
        isFixed = true
        state = .shown(spec)
        self.hasPage = hasPage
        self.hasPDF = hasPDF
        self.values = values
    }

    isolated deinit {
        pending?.cancel()
        if let registration { relay?.unregister(registration) }
    }

    /// Whether the core has a value for any bound widget.
    public var isLive: Bool { values.values.contains { $0.missing.isEmpty } }

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
        await readMeasures()
    }

    /// The measure ids the spec's widgets are bound to, once each, in order.
    nonisolated static func boundIDs(of spec: Deliverable) -> [String] {
        var seen = Set<String>()
        return spec.sections.flatMap(\.widgets).compactMap(\.measure).filter {
            seen.insert($0).inserted
        }
    }

    /// Reads the bound measures from the core. A failed read keeps what was
    /// last read; no bound id makes no call.
    func readMeasures() async {
        guard let client, case .shown(let spec) = state else { return }
        let ids = Self.boundIDs(of: spec)
        guard !ids.isEmpty else {
            values = [:]
            return
        }
        var read: [String: Hearth_V1_MeasureValue] = [:]
        do {
            for start in stride(from: 0, to: ids.count, by: Self.readCap) {
                let chunk = Array(ids[start..<min(start + Self.readCap, ids.count)])
                for value in try await client.readMeasures(ids: chunk) { read[value.id] = value }
            }
        } catch {
            return
        }
        values = read
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .projectChanged, .usageTick, .inboxChanged, .noteChanged, .handbookChanged:
            guard client != nil, !values.isEmpty || isShownBound else { return }
            pending?.cancel()
            pending = Task { [weak self, debounce] in
                try? await Task.sleep(for: debounce)
                guard !Task.isCancelled else { return }
                await self?.readMeasures()
            }
        case .resync:
            pending?.cancel()
            await readMeasures()
        default:
            break
        }
    }

    /// Waits for a read an event scheduled, for tests.
    func settle() async { await pending?.value }

    private var isShownBound: Bool {
        guard case .shown(let spec) = state else { return false }
        return !Self.boundIDs(of: spec).isEmpty
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
