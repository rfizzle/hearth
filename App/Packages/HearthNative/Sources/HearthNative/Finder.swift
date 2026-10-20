// Finder shows a file the core wrote: it opens a Finder window on the file's
// folder with the file selected, through NSWorkspace. The app target hands it
// to the views as the revealInFinder environment action.

import AppKit

/// The Finder, as the app reaches it.
public enum Finder {
    /// Opens Finder with the file at the absolute `path` selected.
    @MainActor
    public static func reveal(path: String) {
        NSWorkspace.shared.activateFileViewerSelecting([URL(filePath: path)])
    }
}
