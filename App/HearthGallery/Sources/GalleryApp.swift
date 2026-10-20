// GalleryApp shows every component in every PreviewState, so a unit that
// declares no states does not reach the gallery and the build says so.

import HearthDesign
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
                // Hire: TemplateCard and TierPicker, light and dark.
                ForEach([ColorScheme.light, .dark], id: \.self) { scheme in
                    Section("TemplateCard, \(scheme == .light ? "light" : "dark")") {
                        ForEach(TemplateCard.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) {
                                TemplateCard(previewState: state)
                            }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("TierPicker, \(scheme == .light ? "light" : "dark")") {
                        ForEach(TierPicker.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) {
                                TierPicker(previewState: state)
                            }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                }
                // End hire.
                // Conversation: Avatar, MessageRow, ToolCallRow, FileRow,
                // RunStatusLine and Composer, light and dark.
                ForEach([ColorScheme.light, .dark], id: \.self) { scheme in
                    let appearance = scheme == .light ? "light" : "dark"
                    Section("Avatar, \(appearance)") {
                        ForEach(Avatar.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { Avatar(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("MessageRow, \(appearance)") {
                        ForEach(MessageRow.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { MessageRow(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("ToolCallRow, \(appearance)") {
                        ForEach(ToolCallRow.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { ToolCallRow(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("FileRow, \(appearance)") {
                        ForEach(FileRow.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { FileRow(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("RunStatusLine, \(appearance)") {
                        ForEach(RunStatusLine.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { RunStatusLine(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                    Section("Composer, \(appearance)") {
                        ForEach(Composer.PreviewState.allCases, id: \.self) { state in
                            LabeledContent(state.rawValue) { Composer(previewState: state) }
                        }
                    }
                    .environment(\.colorScheme, scheme)
                }
                // End conversation.
            }
        }
    }
}
