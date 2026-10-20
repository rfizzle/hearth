// FolderPicker asks the owner for one folder, or one file, through the
// system's open panel: one selection, folders, files or both as asked. The
// build is not sandboxed, so the path is all that is kept; no
// security-scoped bookmark. The app target hands it to the views as the
// pickFolder environment action.

import AppKit

/// The open panel, as the app reaches it.
public enum FolderPicker {
    /// Shows the panel for a folder and answers the chosen folder's absolute
    /// path, or nil when the owner cancels.
    @MainActor
    public static func pick() async -> String? {
        await show(folders: true, files: false, start: nil, prompt: "Add")
    }

    /// Shows the panel choosing folders, files or both (folders when neither
    /// is asked), opened at `start` when it is given, and answers the chosen
    /// absolute path, or nil when the owner cancels.
    @MainActor
    public static func pickPath(folders: Bool, files: Bool, start: String?) async -> String? {
        await show(folders: folders, files: files, start: start, prompt: "Choose")
    }

    @MainActor
    private static func show(
        folders: Bool, files: Bool, start: String?, prompt: String
    ) async -> String? {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = folders || !files
        panel.canChooseFiles = files
        panel.allowsMultipleSelection = false
        panel.prompt = prompt
        if let start, !start.isEmpty {
            panel.directoryURL = URL(filePath: start, directoryHint: .isDirectory)
        }
        let response: NSApplication.ModalResponse =
            if let window = NSApp.keyWindow {
                await panel.beginSheetModal(for: window)
            } else {
                panel.runModal()
            }
        return response == .OK ? panel.url?.path(percentEncoded: false) : nil
    }
}
