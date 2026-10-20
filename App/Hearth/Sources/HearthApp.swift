// HearthApp is the app's entry point and its wiring: the app delegate owns the
// helper supervisor so the helper is stopped when the app quits, and the model
// that builds the stores over each helper. One main window, a Settings window
// and the Debug menu; the Team menu carries Hire Someone… on ⌘N and Access… on ⌥⌘A,
// View carries Show Inspector on ⌥⌘I, and File carries New Project… on ⇧⌘N in
// place of the New Window item a window group would put there. The menu bar
// extra and the quick-ask window its global shortcut opens are scenes too, and
// so is the window a shaped deliverable opens in, one per file; the
// delegate starts the helper at launch and listens for the shortcut.
// See docs/design.md#window-anatomy

import AppKit
import HearthFeatures
import HearthNative
import HearthUI
import SwiftUI

@main
struct HearthApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate

    var body: some Scene {
        WindowGroup(id: AppModel.mainWindowID) {
            ContentView()
                .environment(appDelegate.model)
                .environment(\.revealInFinder, RevealInFinderAction { Finder.reveal(path: $0) })
                .environment(\.openInBrowser, OpenInBrowserAction { Browser.open(path: $0) })
                .environment(
                    \.pickFolder,
                    PickFolderAction { folders, files, start in
                        await FolderPicker.pickPath(folders: folders, files: files, start: start)
                    }
                )
                .task { appDelegate.supervisor.start() }
        }
        .defaultSize(width: 1040, height: 700)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("New Project…") { appDelegate.model.isCreatingProject = true }
                    .keyboardShortcut("n", modifiers: [.command, .shift])
                    .disabled(appDelegate.model.session?.phase != .team)
            }
            SidebarCommands()
            CommandGroup(after: .sidebar) {
                Button(appDelegate.model.showsInspector ? "Hide Inspector" : "Show Inspector") {
                    appDelegate.model.showsInspector.toggle()
                }
                .keyboardShortcut("i", modifiers: [.command, .option])
                .disabled(appDelegate.model.session?.sidebar.selectedBot == nil)
            }
            TeamCommands(model: appDelegate.model)
            DebugCommands()
        }
        // One window per deliverable file: opening an equal path again brings
        // the same window forward.
        WindowGroup(id: DeliverableChip.windowID, for: String.self) { $path in
            if let path {
                DeliverableScreen(path: path)
                    .environment(\.revealInFinder, RevealInFinderAction { Finder.reveal(path: $0) })
                    .environment(\.openInBrowser, OpenInBrowserAction { Browser.open(path: $0) })
            }
        }
        .defaultSize(width: 760, height: 640)
        Settings {
            SettingsView().environment(appDelegate.model)
        }
        MenuBarExtraScene(model: appDelegate.model)
        QuickAskScene(model: appDelegate.model)
        ShortcutsDebugWindow()
    }
}

/// The Team menu: every toolbar action also lives in a menu.
struct TeamCommands: Commands {
    let model: AppModel

    var body: some Commands {
        CommandMenu("Team") {
            Button("Hire Someone…") { model.isHiring = true }
                .keyboardShortcut("n")
                .disabled(model.session?.phase != .team)
            Button("Access…") { model.accessBotID = model.session?.sidebar.selectedBot?.id }
                .keyboardShortcut("a", modifiers: [.command, .option])
                .disabled(
                    model.session?.phase != .team || model.session?.sidebar.selectedBot == nil)
        }
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    let supervisor: HelperSupervisor
    let model: AppModel

    override init() {
        let supervisor = HelperSupervisor()
        self.supervisor = supervisor
        model = AppModel(supervisor: supervisor)
        super.init()
    }

    /// Starts the helper and follows it, so the menu bar extra has a session
    /// whether or not the window opens, and listens for the quick ask.
    func applicationDidFinishLaunching(_ notification: Notification) {
        model.follow()
        supervisor.start()
        GlobalShortcut.quickAsk.onPress { [weak self] in self?.model.showQuickAsk() }
    }

    func applicationWillTerminate(_ notification: Notification) {
        GlobalShortcut.quickAsk.stopListening()
        supervisor.stop()
    }
}
