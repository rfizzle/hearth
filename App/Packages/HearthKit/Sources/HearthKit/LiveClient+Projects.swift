// LiveClient's project calls, in a file of their own to keep both short.

import Connect

extension LiveClient {
    public func listProjects(includeArchived: Bool) async throws -> [Hearth_V1_Project] {
        let request = Hearth_V1_ListProjectsRequest.with { $0.includeArchived = includeArchived }
        return try await projects.listProjects(request: request).value().projects
    }

    public func draftProject(description: String) async throws -> Hearth_V1_DraftProjectResponse {
        let request = Hearth_V1_DraftProjectRequest.with { $0.description_p = description }
        return try await projects.draftProject(request: request).value()
    }

    public func checkProject(
        _ request: Hearth_V1_CheckProjectRequest
    ) async throws -> [Hearth_V1_Concern] {
        try await projects.checkProject(request: request).value().concerns
    }

    public func createProject(
        _ request: Hearth_V1_CreateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await projects.createProject(request: request).value().project
    }

    public func updateProject(
        _ request: Hearth_V1_UpdateProjectRequest
    ) async throws -> Hearth_V1_Project {
        try await projects.updateProject(request: request).value().project
    }

    public func markProjectDone(id: String) async throws -> Hearth_V1_Project {
        let request = Hearth_V1_MarkProjectDoneRequest.with { $0.id = id }
        return try await projects.markProjectDone(request: request).value().project
    }

    public func archiveProject(id: String) async throws -> Hearth_V1_Project {
        let request = Hearth_V1_ArchiveProjectRequest.with { $0.id = id }
        return try await projects.archiveProject(request: request).value().project
    }
}

extension HearthClientError {
    /// Whether `error` is the core saying it cannot do this now (`unavailable`):
    /// no model is signed in, or the provider is down. Its words are the
    /// error's description.
    public static func isUnavailable(_ error: any Error) -> Bool {
        if case .unavailable? = error as? HearthClientError { return true }
        return (error as? ConnectError)?.code == .unavailable
    }
}
