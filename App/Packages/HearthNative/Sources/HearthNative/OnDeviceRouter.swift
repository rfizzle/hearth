import Foundation
import FoundationModels

/// The on-device model's guess at which team should take a request.
public struct RouteHint: Sendable, Equatable {
    /// One of the names passed to the router, never anything else.
    public let team: String

    public init(team: String) {
        self.team = team
    }
}

/// Picks a team for a request with the on-device model. The answer is a hint:
/// nil means "ask the fast tier", whether the model is unavailable, the request
/// is empty, or the call failed.
public struct OnDeviceRouter: Sendable {
    private let availability: OnDeviceAvailability

    public init(availability: OnDeviceAvailability = .system) {
        self.availability = availability
    }

    public func route(request: String, teams: [String]) async -> RouteHint? {
        guard availability.isAvailable(), !teams.isEmpty else { return nil }
        let trimmed = request.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return nil }

        // The team list is only known at run time, so the enum a @Generable
        // type would declare is built as a dynamic schema: a string that must
        // be one of the team names. The model cannot answer outside the list.
        let choices = DynamicGenerationSchema(name: "Team", anyOf: teams)
        guard let schema = try? GenerationSchema(root: choices, dependencies: []) else {
            return nil
        }
        let session = LanguageModelSession(
            instructions:
                "Choose the one team best suited to the request. Answer with the team name only."
        )
        let prompt = "Teams: \(teams.joined(separator: ", "))\nRequest: \(trimmed)"
        guard let response = try? await session.respond(to: prompt, schema: schema),
            let name = try? response.content.value(String.self),
            teams.contains(name)
        else { return nil }
        return RouteHint(team: name)
    }
}
