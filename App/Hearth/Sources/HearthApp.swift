// HearthApp is the app's entry point and its wiring: the app delegate owns the
// helper supervisor so the helper is stopped when the app quits, and the
// window gets the supervisor and the hello store through the environment.

import AppKit
import HearthFeatures
import SwiftUI

@main
struct HearthApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @State private var store = HelloStore()

    var body: some Scene {
        WindowGroup {
            ContentView()
                .environment(appDelegate.supervisor)
                .environment(store)
                .task { appDelegate.supervisor.start() }
        }
        .commands { DebugCommands() }
        ShortcutsDebugWindow()
    }
}

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    let supervisor = HelperSupervisor()

    func applicationWillTerminate(_ notification: Notification) {
        supervisor.stop()
    }
}
