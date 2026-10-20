// ContentView is the main window's content. With no helper it says what the
// helper is doing; with one, it asks the core for the org and shows the
// onboarding column until there is one, then cross-fades to the team. A
// cross-fade is not motion, so Reduce Motion keeps it but the animation is
// dropped there. The window is titled by what it shows: Welcome, the org, or
// the selected bot; never the app's name. The model builds the session; the
// window hands it the way to open windows, for the global shortcut.
// See docs/design.md#window-anatomy, docs/design.md#empty-states-and-first-run
// and docs/design.md#motion

import HearthDesign
import HearthFeatures
import SwiftUI

struct ContentView: View {
    @Environment(AppModel.self) private var model
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        Group {
            if let session = model.session {
                SessionView(session: session)
            } else {
                VStack(spacing: Theme.Spacing.md) {
                    ProgressView()
                    Text(model.supervisor.status)
                        .font(Theme.callout)
                        .foregroundStyle(Theme.color(.textSecondary))
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
            }
        }
        .frame(minWidth: 760, minHeight: 480)
        .onAppear { model.windowOpener = openWindow }
    }
}

/// What the window shows over one helper, by phase.
private struct SessionView: View {
    let session: Session
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    var body: some View {
        ZStack {
            switch session.phase {
            case .loading:
                ProgressView().transition(.opacity)
            case .failed(let message):
                ContentUnavailableView {
                    Label("Couldn't reach your team", systemImage: "exclamationmark.triangle")
                } description: {
                    Text(message)
                } actions: {
                    Button("Try Again") { Task { await session.loadOrg() } }
                }
                .transition(.opacity)
            case .onboarding:
                OnboardingScreen(store: session.onboarding).transition(.opacity)
            case .team:
                TeamView(session: session).transition(.opacity)
            }
        }
        .animation(Theme.motion(Theme.quick, reduceMotion: reduceMotion), value: session.phase)
        .task { await session.loadOrg() }
    }
}
