// HelloScreen shows the hello store's transcript in a TranscriptText, and the
// stream's failure under it when there is one.

import HearthUI
import SwiftUI

/// The screen that shows the streamed words.
public struct HelloScreen: View {
    let store: HelloStore

    public init(store: HelloStore) {
        self.store = store
    }

    public var body: some View {
        VStack(alignment: .leading) {
            TranscriptText(text: store.transcript)
            if let error = store.error {
                TranscriptText(text: error)
            }
        }
        .padding()
    }
}
