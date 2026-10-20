// Browser opens a page the core wrote in the owner's default browser, through
// NSWorkspace, as Finder shows a file. It is the system opening a file the
// owner chose; the app never shows the page in a web view. The app target
// hands it to the views as the openInBrowser environment action.

import AppKit

/// The default browser, as the app reaches it.
public enum Browser {
    /// Whether `path` is a page worth handing to the browser: an absolute path
    /// to a `.html` file, written without `..` or `.` parts.
    public static func isPage(path: String) -> Bool {
        let url = URL(filePath: path)
        return url.pathExtension.lowercased() == "html"
            && path.hasPrefix("/")
            && url.standardizedFileURL.path == path
    }

    /// Opens the page at the absolute `path` in the default browser; any
    /// other path, or a file that is gone, is left alone.
    @MainActor
    public static func open(path: String) {
        guard isPage(path: path), FileManager.default.fileExists(atPath: path) else { return }
        NSWorkspace.shared.open(URL(filePath: path))
    }
}
