// The rows and controls a conversation is built from.

import HearthUI

extension Catalog {
    static var conversation: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "MessageRow",
                states: GalleryState.each(MessageRow.PreviewState.self, width: 560) {
                    MessageRow(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "ToolCallRow",
                states: GalleryState.each(ToolCallRow.PreviewState.self, width: 560) {
                    ToolCallRow(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "FileRow",
                states: GalleryState.each(FileRow.PreviewState.self, width: 560) {
                    FileRow(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "AttachmentChip",
                states: GalleryState.each(AttachmentChip.PreviewState.self, width: 560) {
                    AttachmentChip(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "RunStatusLine",
                states: GalleryState.each(RunStatusLine.PreviewState.self, width: 560) {
                    RunStatusLine(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "HelperBanner",
                states: GalleryState.each(HelperBanner.PreviewState.self, width: 760) {
                    HelperBanner(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "Composer",
                states: GalleryState.each(Composer.PreviewState.self, width: 560) {
                    Composer(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "TranscriptText",
                states: GalleryState.each(TranscriptText.PreviewState.self, width: 560) {
                    TranscriptText(text: $0.text)
                }),
        ]
    }
}
