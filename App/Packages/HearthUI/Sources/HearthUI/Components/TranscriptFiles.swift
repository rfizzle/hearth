// TranscriptFiles is the files a message or a tool result produced: a FileRow
// for each, except that the files of one shaped deliverable are one chip.

import HearthKit
import SwiftUI

/// The files under a message, a deliverable's files as one chip.
struct TranscriptFiles: View {
    let files: [Hearth_V1_FileRef]

    var body: some View {
        ForEach(DeliverableGroup.items(files), id: \.id) { item in
            switch item {
            case .file(let file): FileRow(file: file)
            case .deliverable(let group): DeliverableChip(group: group)
            }
        }
    }
}
