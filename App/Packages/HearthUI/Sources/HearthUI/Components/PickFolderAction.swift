// PickFolderAction is how a view asks the owner to choose a folder. The app
// target sets it in the environment from HearthNative, so HearthUI and the
// features never import the native adapters; the default chooses nothing,
// which is what the gallery and previews want.

import SwiftUI

/// Asks the owner to choose a folder and answers its absolute path.
public struct PickFolderAction: Sendable {
    let action: @MainActor @Sendable () async -> String?

    /// An action that answers the chosen folder's path, or nil if none was.
    public init(_ action: @escaping @MainActor @Sendable () async -> String?) {
        self.action = action
    }

    /// Shows the picker; nil when the owner cancels.
    @MainActor
    public func callAsFunction() async -> String? {
        await action()
    }
}

extension EnvironmentValues {
    /// Chooses a folder; set by the app target.
    @Entry public var pickFolder = PickFolderAction { nil }
}
