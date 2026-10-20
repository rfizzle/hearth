// GlobalShortcut is a key combination that reaches hearth from any app: the
// quick ask, ⌃⌥Space unless the owner records another in Settings. It wraps
// the KeyboardShortcuts package, which registers a Carbon hot key (no
// Accessibility permission, no dialog) and keeps the owner's choice in the
// app's defaults; nothing above this file imports the package. The recorder is
// the package's own control, so its keyboard and VoiceOver behaviour are the
// system's.

import AppKit
import KeyboardShortcuts
import SwiftUI

/// A named system-wide shortcut.
public struct GlobalShortcut: Sendable {
    let name: KeyboardShortcuts.Name

    /// The quick ask: opens the one-line field from anywhere. Its default is
    /// ⌃⌥Space.
    public static let quickAsk = GlobalShortcut(
        name: KeyboardShortcuts.Name(
            "quickAsk",
            initial: KeyboardShortcuts.Shortcut(.space, modifiers: [.control, .option])))

    /// The combination it starts with, as a menu shows it.
    @MainActor
    public var defaultDescription: String? {
        name.initialShortcut?.description
    }

    /// Calls `action` each time the owner presses the shortcut, until
    /// `stopListening`; the hot key is registered while anything listens.
    @MainActor
    public func onPress(_ action: @escaping @MainActor () -> Void) {
        KeyboardShortcuts.onKeyDown(for: name) { action() }
    }

    /// Stops calling every action and lets the hot key go.
    @MainActor
    public func stopListening() {
        KeyboardShortcuts.removeHandler(for: name)
    }

    /// A control that records a new combination for the shortcut, titled
    /// `title`.
    @MainActor
    public func recorder(_ title: String) -> some View {
        KeyboardShortcuts.Recorder(title, name: name)
    }
}
