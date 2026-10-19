// OnboardingStore walks the owner through Welcome, Sign in, and Name and
// folder, then sends the name and folder to the core. It keeps the step, the
// draft name, the chosen folder and the last error; whether a name or folder
// is acceptable is the core's call, and it answers in an error shown here.

import Foundation
import HearthKit
import Observation

/// The store behind the onboarding screen.
@MainActor
@Observable
public final class OnboardingStore {
    /// The three steps, in order.
    public enum Step: Int, CaseIterable, Sendable {
        case welcome
        case signIn
        case nameAndFolder
    }

    /// The accounts the sign-in step shows.
    public let accounts: AccountsStore
    /// The step on screen.
    public private(set) var step = Step.welcome
    /// The org name as typed.
    public var name = ""
    /// The Hearth folder, as a path; `~/Documents/Hearth` until one is chosen.
    public var path: String
    /// The last failed call's text, if one failed.
    public private(set) var error: String?
    /// The org the core created, once Done has succeeded.
    public private(set) var org: Hearth_V1_Org?

    private let client: any HearthClient

    /// A store over `client` and `accounts`; `home` is where the default
    /// folder sits.
    public init(
        client: any HearthClient, accounts: AccountsStore,
        home: URL = FileManager.default.homeDirectoryForCurrentUser
    ) {
        self.client = client
        self.accounts = accounts
        path = home.appending(path: "Documents/Hearth").path
    }

    /// Whether Continue is enabled on the current step. The sign-in step needs
    /// a provider account, because the team cannot think without a model; an
    /// integration alone never opens it. See docs/model.md#providers-and-models
    public var canContinue: Bool {
        switch step {
        case .welcome: true
        case .signIn: accounts.hasSignedInAccount
        case .nameAndFolder: false
        }
    }

    /// Whether Done is enabled: a name has been typed.
    public var canFinish: Bool {
        step == .nameAndFolder && !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }

    /// Moves to the next step when Continue is enabled.
    public func next() {
        guard canContinue, let following = Step(rawValue: step.rawValue + 1) else { return }
        step = following
    }

    /// Moves to the previous step.
    public func back() {
        if let previous = Step(rawValue: step.rawValue - 1) { step = previous }
    }

    /// Sends the name and folder to the core, which creates the org.
    public func done() async {
        guard canFinish else { return }
        do {
            org = try await client.updateOrg(name: name, workspacePath: path)
            error = nil
        } catch {
            self.error = error.localizedDescription
        }
    }
}
