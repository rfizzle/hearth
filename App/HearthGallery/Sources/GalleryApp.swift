// GalleryApp shows every component in every PreviewState, so a unit that
// declares no states does not reach the gallery and the build says so.

import HearthUI
import SwiftUI

@main
struct GalleryApp: App {
    var body: some Scene {
        WindowGroup("Hearth Gallery") {
            List {
                Section("TranscriptText") {
                    ForEach(TranscriptText.PreviewState.allCases, id: \.self) { state in
                        LabeledContent(state.rawValue) {
                            TranscriptText(text: state.text)
                        }
                    }
                }
                // Onboarding: SignInPanel and IntegrationsPanel, light and dark.
                ForEach([ColorScheme.light, .dark], id: \.self) { scheme in
                    Section("SignInPanel, \(scheme == .light ? "light" : "dark")") {
                        ForEach(SignInPanel.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) {
                                SignInPanel(previewState: state)
                            }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("IntegrationsPanel, \(scheme == .light ? "light" : "dark")") {
                        ForEach(IntegrationsPanel.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) {
                                IntegrationsPanel(previewState: state)
                            }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                }
                // End onboarding.
            }
        }
    }
}
