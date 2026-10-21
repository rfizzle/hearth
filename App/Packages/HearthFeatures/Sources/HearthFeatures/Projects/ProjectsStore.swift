// ProjectsStore holds the org's projects as the core last reported them:
// proposed, active and done, never archived. It fetches the list, applies the
// project changes the relay delivers (an archived one leaves the list),
// refetches on a resync, and turns the owner's create, edit, done and archive
// into calls; each answer replaces the project it names. It decides nothing:
// the status and its light are the core's, and the sidebar lists the active
// projects first and the done ones after, in the core's order within each.
// See docs/model.md#work

import HearthKit
import Observation

/// The store behind the sidebar's projects and the project sheet.
@MainActor
@Observable
public final class ProjectsStore {
    /// Every proposed, active and done project, in the order the core sent
    /// them; new ones come last.
    public private(set) var projects: [Hearth_V1_Project] = []
    /// Whether the list has loaded once, so an empty one is known to be empty.
    public private(set) var loaded = false
    /// The last failed load, done or archive's text, if one failed.
    public private(set) var error: String?
    /// The last failed create or edit's text, shown under the sheet's form.
    public private(set) var saveError: String?
    /// Whether a create or an edit is in flight.
    public private(set) var isSaving = false

    private let client: any HearthClient
    @ObservationIgnored private let relay: EventRelay
    @ObservationIgnored private var registration: EventRelay.Registration?

    /// A store over `client`, fed by `relay`.
    public init(client: any HearthClient, relay: EventRelay) {
        self.client = client
        self.relay = relay
        registration = relay.register { [weak self] event in await self?.handle(event) }
    }

    isolated deinit {
        if let registration { relay.unregister(registration) }
    }

    /// The sidebar's projects: the active ones, then the done ones.
    public var listed: [Hearth_V1_Project] {
        projects.filter { $0.state == .active } + projects.filter { $0.state == .done }
    }

    /// The proposals waiting for the owner's click.
    public var proposals: [Hearth_V1_Project] {
        projects.filter { $0.state == .proposed }
    }

    /// The listed project with `id`, if it is still listed.
    public func project(id: String) -> Hearth_V1_Project? {
        listed.first { $0.id == id }
    }

    /// Fetches the list from the core.
    public func load() async {
        do {
            projects = try await client.listProjects(includeArchived: false)
            loaded = true
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    /// Forgets the last failed create or edit, as a sheet opens.
    public func clearSaveError() {
        saveError = nil
    }

    /// Creates a project; nil when the core refused it, with its words in
    /// `saveError`.
    public func create(_ request: Hearth_V1_CreateProjectRequest) async -> Hearth_V1_Project? {
        await save { try await self.client.createProject(request) }
    }

    /// Replaces a project's name, brief and target date; nil when the core
    /// refused it, with its words in `saveError`.
    public func update(_ request: Hearth_V1_UpdateProjectRequest) async -> Hearth_V1_Project? {
        await save { try await self.client.updateProject(request) }
    }

    /// Confirms a proposal as it stands: the core makes it an active project
    /// with the same id. Nil when the core refused, with its words in `error`.
    public func confirm(_ proposal: Hearth_V1_Project) async -> Hearth_V1_Project? {
        let request = Hearth_V1_CreateProjectRequest.with {
            $0.proposalID = proposal.id
            $0.name = proposal.name
            $0.brief = proposal.brief
            $0.targetDate = proposal.targetDate
        }
        do {
            let project = try await client.createProject(request)
            upsert(project)
            error = nil
            return project
        } catch {
            self.error = error.localizedDescription
            return nil
        }
    }

    /// Sets a proposal aside: it is archived and leaves the list.
    public func dismiss(_ proposal: Hearth_V1_Project) async {
        await archive(id: proposal.id)
    }

    /// The proposals made in the conversation with `conversationID`, oldest
    /// first, so the newest sits last.
    public func proposals(in conversationID: String) -> [Hearth_V1_Project] {
        proposals.filter { $0.proposedInConversationID == conversationID }
    }

    /// The owner's mark that the project with `id` is done.
    public func markDone(id: String) async {
        await change { try await self.client.markProjectDone(id: id) }
    }

    /// Archives the project with `id`; it leaves the list.
    public func archive(id: String) async {
        await change { try await self.client.archiveProject(id: id) }
    }

    /// Applies one event from the relay.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        switch event.event {
        case .projectChanged(let change): upsert(change.project)
        case .resync: await load()
        default: break
        }
    }

    private func save(
        _ call: () async throws -> Hearth_V1_Project
    ) async -> Hearth_V1_Project? {
        isSaving = true
        defer { isSaving = false }
        do {
            let project = try await call()
            upsert(project)
            saveError = nil
            return project
        } catch {
            saveError = error.localizedDescription
            return nil
        }
    }

    private func change(_ call: () async throws -> Hearth_V1_Project) async {
        do {
            upsert(try await call())
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }

    private func upsert(_ project: Hearth_V1_Project) {
        let index = projects.firstIndex { $0.id == project.id }
        switch (index, project.state == .archived) {
        case (let index?, true): projects.remove(at: index)
        case (let index?, false): projects[index] = project
        case (nil, false): projects.append(project)
        case (nil, true): break
        }
    }
}
