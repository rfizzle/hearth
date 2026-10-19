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
            }
        }
    }
}
