// PreviewClient's project calls and the list they change, in a file of their
// own to keep both short.

import Foundation

extension PreviewClient {
    public func listProjects(includeArchived: Bool) async throws -> [Hearth_V1_Project] {
        projectList.all().filter { includeArchived || $0.state != .archived }
    }

    public func createProject(
        _ request: Hearth_V1_CreateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try projectList.create(request)
    }

    public func updateProject(
        _ request: Hearth_V1_UpdateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try projectList.change(request.id) {
            $0.name = request.name
            $0.brief = request.brief
            $0.targetDate = request.targetDate
        }
    }

    public func markProjectDone(id: String) async throws -> Hearth_V1_Project {
        try projectList.change(id) {
            $0.state = .done
            $0.doneAt = Fixtures.time(60)
        }
    }

    public func archiveProject(id: String) async throws -> Hearth_V1_Project {
        try projectList.change(id) {
            $0.state = .archived
            $0.archivedAt = Fixtures.time(60)
        }
    }
}

/// The projects PreviewClient serves, kept so a write shows in the next read.
final class ProjectList: @unchecked Sendable {
    private let lock = NSLock()
    private var projects = Fixtures.projects
    private var made = 0

    func all() -> [Hearth_V1_Project] {
        lock.withLock { projects }
    }

    /// An active project, or the proposal `request.proposalID` names, confirmed.
    /// A name over 80 characters is refused in the core's words.
    func create(_ request: Hearth_V1_CreateProjectRequest) throws -> Hearth_V1_Project {
        guard request.name.count <= 80 else {
            throw PageRefusal(message: "invalid project: the name is over 80 characters")
        }
        return try lock.withLock {
            var project: Hearth_V1_Project
            if request.proposalID.isEmpty {
                made += 1
                project = .with { $0.id = "project-new-\(made)" }
            } else if let found = projects.first(where: { $0.id == request.proposalID }) {
                project = found
            } else {
                throw PreviewError.projectNotFound(request.proposalID)
            }
            if !request.name.isEmpty { project.name = request.name }
            if request.hasBrief { project.brief = request.brief }
            if !request.targetDate.isEmpty { project.targetDate = request.targetDate }
            project.state = .active
            project.folderPath = Fixtures.workspacePath + "/Projects/" + project.name
            project.conversationID = "conversation-" + project.id
            project.activatedAt = Fixtures.time(60)
            if !project.hasCreatedAt { project.createdAt = Fixtures.time(60) }
            project.updatedAt = Fixtures.time(60)
            project.status = Fixtures.noTasks
            if let index = projects.firstIndex(where: { $0.id == project.id }) {
                projects[index] = project
            } else {
                projects.append(project)
            }
            return project
        }
    }

    func change(
        _ id: String, _ edit: (inout Hearth_V1_Project) -> Void
    ) throws -> Hearth_V1_Project {
        try lock.withLock {
            guard let index = projects.firstIndex(where: { $0.id == id }) else {
                throw PreviewError.projectNotFound(id)
            }
            edit(&projects[index])
            projects[index].updatedAt = Fixtures.time(60)
            return projects[index]
        }
    }
}

/// What the preview client refuses.
enum PreviewError: Error, Equatable {
    case projectNotFound(String)
}
