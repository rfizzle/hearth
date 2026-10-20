// OpenInBrowserAction is how a view asks for a page the core wrote to be opened
// in the owner's browser. The app target sets it in the environment from
// HearthNative, as it does the Finder action; the default does nothing, which
// is what the gallery and previews want. The app never shows the page itself.

import SwiftUI

/// Opens the web page at a path in the default browser.
public struct OpenInBrowserAction: Sendable {
    let action: @MainActor @Sendable (String) -> Void

    /// An action that calls `action` with the page's absolute path.
    public init(_ action: @escaping @MainActor @Sendable (String) -> Void) {
        self.action = action
    }

    /// Opens the page at `path`.
    @MainActor
    public func callAsFunction(_ path: String) {
        action(path)
    }
}

extension EnvironmentValues {
    /// Opens a page in the browser; set by the app target.
    @Entry public var openInBrowser = OpenInBrowserAction { _ in }
}
