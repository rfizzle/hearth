// ShortcutsDebug is a spike surface, compiled into debug builds only: a Debug
// menu that opens a window listing the user's shortcuts and running one with
// typed input, showing each state the run passes through. Release builds
// carry none of it.

#if DEBUG

    import HearthDesign
    import HearthNative
    import SwiftUI
    import os

    struct DebugCommands: Commands {
        @Environment(\.openWindow) private var openWindow

        var body: some Commands {
            CommandMenu("Debug") {
                Button("Shortcuts…") { openWindow(id: ShortcutsDebugView.windowID) }
                    .keyboardShortcut("s", modifiers: [.command, .option])
            }
        }
    }

    @MainActor
    @Observable
    final class ShortcutsDebugModel {
        var names: [String] = []
        var selection: String?
        var input = ""
        var transitions: [String] = []
        var output = ""
        var error = ""
        var busy = false

        private let runner = ShortcutsRunner(grace: .seconds(5))
        private let log = Logger(subsystem: "com.rfizzle.hearth", category: "shortcuts")

        func refresh() async {
            do {
                names = try await runner.list()
                log.notice("listed \(self.names.count) shortcuts")
                error = ""
            } catch {
                self.error = "\(error)"
                log.notice("list failed: \(error)")
            }
        }

        func run() async {
            guard let name = selection, !busy else { return }
            busy = true
            transitions = []
            output = ""
            error = ""
            let started = ContinuousClock.now
            let record: @Sendable (ShortcutsState) -> Void = { [weak self] state in
                Task { @MainActor in
                    self?.transitions.append("\(state)")
                    self?.log.notice("state \(String(describing: state))")
                }
            }
            do {
                output = try await runner.run(
                    name: name, input: input, timeout: .seconds(30), onState: record)
            } catch {
                self.error = "\(error)"
            }
            log.notice("run took \(started.duration(to: .now).description)")
            busy = false
        }
    }

    struct ShortcutsDebugView: View {
        static let windowID = "shortcuts-debug"
        @State private var model = ShortcutsDebugModel()

        var body: some View {
            VStack(alignment: .leading, spacing: Theme.Spacing.sm) {
                HStack {
                    Picker("Shortcut", selection: $model.selection) {
                        Text("None").tag(String?.none)
                        ForEach(model.names, id: \.self) { Text($0).tag(String?.some($0)) }
                    }
                    Button("Refresh") { Task { await model.refresh() } }
                }
                TextField("Input", text: $model.input)
                Button("Run") { Task { await model.run() } }
                    .disabled(model.selection == nil || model.busy)
                Text("States: " + model.transitions.joined(separator: " → "))
                if !model.output.isEmpty { Text("Output: \(model.output)").textSelection(.enabled) }
                if !model.error.isEmpty { Text("Error: \(model.error)").foregroundStyle(.red) }
            }
            .padding()
            .frame(minWidth: 420, minHeight: 240, alignment: .topLeading)
            .task { await model.refresh() }
        }
    }

    struct ShortcutsDebugWindow: Scene {
        var body: some Scene {
            Window("Shortcuts", id: ShortcutsDebugView.windowID) { ShortcutsDebugView() }
        }
    }

#else

    import SwiftUI

    // Release builds keep the call sites in HearthApp and none of the surface.
    struct DebugCommands: Commands {
        var body: some Commands {}
    }

    struct ShortcutsDebugWindow: Scene {
        var body: some Scene {
            WindowGroup(id: "unused-debug") { EmptyView() }.defaultLaunchBehavior(.suppressed)
        }
    }

#endif
