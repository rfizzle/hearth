// FolderPicker asks the owner for one folder through the system's open panel:
// directories only, no files, one selection. The build is not sandboxed, so
// the path is all that is kept; no security-scoped bookmark. The app target
// hands it to the views as the pickFolder environment action.

import AppKit

/// The folder panel, as the app reaches it.
public enum FolderPicker {
    /// Shows the panel and answers the chosen folder's absolute path, or nil
    /// when the owner cancels.
    @MainActor
    public static func pick() async -> String? {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        panel.prompt = "Add"
        let response: NSApplication.ModalResponse =
            if let window = NSApp.keyWindow {
                await panel.beginSheetModal(for: window)
            } else {
                panel.runModal()
            }
        return response == .OK ? panel.url?.path(percentEncoded: false) : nil
    }
}
