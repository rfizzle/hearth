// RevealInFinderAction is how a view asks for a file to be shown in Finder.
// The app target sets it in the environment from HearthNative, so HearthUI and
// the features never import the native adapters; the default does nothing,
// which is what the gallery and previews want.

import SwiftUI

/// Shows the file at a path in Finder.
public struct RevealInFinderAction: Sendable {
    let action: @MainActor @Sendable (String) -> Void

    /// An action that calls `action` with the file's absolute path.
    public init(_ action: @escaping @MainActor @Sendable (String) -> Void) {
        self.action = action
    }

    /// Shows the file at `path`.
    @MainActor
    public func callAsFunction(_ path: String) {
        action(path)
    }
}

extension EnvironmentValues {
    /// Shows a file in Finder; set by the app target.
    @Entry public var revealInFinder = RevealInFinderAction { _ in }
}
