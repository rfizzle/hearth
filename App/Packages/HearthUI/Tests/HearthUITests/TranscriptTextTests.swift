// Placeholder test for HearthUI: TranscriptText declares its three states.

import Testing

@testable import HearthUI

@Test func transcriptTextHasThreePreviewStates() {
    #expect(TranscriptText.PreviewState.allCases.count == 3)
}
