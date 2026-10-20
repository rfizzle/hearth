// PickFolderAction is how a view asks the owner to choose a folder or, for a
// question that asks for one, a file. The app target sets it in the
// environment from HearthNative, so HearthUI and the features never import
// the native adapters; the default chooses nothing, which is what the gallery
// and previews want.

import SwiftUI

/// Asks the owner to choose a folder or a file and answers its absolute path.
public struct PickFolderAction: Sendable {
    /// Shows a panel for folders, files or both, opened at a start path.
    public typealias Pick =
        @MainActor @Sendable (
            _ folders: Bool, _ files: Bool, _ start: String?
        ) async -> String?

    let action: Pick

    /// An action that shows a panel choosing folders, files or both, opened at
    /// the start path when one is given, and answers the chosen path, or nil
    /// if none was.
    public init(_ action: @escaping Pick) {
        self.action = action
    }

    /// Shows the folder picker; nil when the owner cancels.
    @MainActor
    public func callAsFunction() async -> String? {
        await action(true, false, nil)
    }

    /// Shows a picker for folders, files or both, opened at `start`; nil when
    /// the owner cancels.
    @MainActor
    public func pickPath(folders: Bool, files: Bool, start: String?) async -> String? {
        await action(folders, files, start)
    }
}

extension EnvironmentValues {
    /// Chooses a folder or a file; set by the app target.
    @Entry public var pickFolder = PickFolderAction { _, _, _ in nil }
}
