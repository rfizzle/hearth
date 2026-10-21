// The rows and controls a conversation is built from.

import CoreGraphics
import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

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
                package: package, name: "RunTrace",
                states: GalleryState.each(RunTrace.PreviewState.self, width: 560) {
                    RunTrace(previewState: $0)
                } + [Self.stillWorking]),
            GalleryPage(
                package: package, name: "JumpToLatestButton",
                states: GalleryState.each(JumpToLatestButton.PreviewState.self, width: 560) {
                    JumpToLatestButton(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "FileRow",
                states: GalleryState.each(FileRow.PreviewState.self, width: 560) { state in
                    let thumbnail = state == .thumbnail ? Self.sampleThumbnail : nil
                    FileRow(previewState: state)
                        .environment(\.fileThumbnail, FileThumbnailAction { _, _ in thumbnail })
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
                package: package, name: "SuggestionChip",
                states: GalleryState.each(SuggestionChip.PreviewState.self, width: 560) {
                    SuggestionChip(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "IntroductionCard",
                states: GalleryState.each(IntroductionCard.PreviewState.self, width: 560) {
                    IntroductionCard(previewState: $0)
                }
                    + GalleryState.each(AboutRow.PreviewState.self, width: 560) {
                        AboutRow(previewState: $0)
                    }),
            GalleryPage(
                package: package, name: "TranscriptText",
                states: GalleryState.each(TranscriptText.PreviewState.self, width: 560) {
                    TranscriptText(text: $0.text)
                }),
        ]
    }
}

extension Catalog {
    /// The same run while it works: not folded, every row as it arrived.
    static var stillWorking: GalleryState {
        let messages = Fixtures.dmMessages
        return GalleryState(
            name: "stillWorking", width: 560,
            view: AnyView(
                VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                    ToolCallRow(call: messages[3].tool, result: messages[4])
                    ToolCallRow(call: messages[5].tool, result: messages[6])
                    ToolCallRow(call: messages[7].tool, result: nil)
                }))
    }

    /// A page of text lines, standing in for what Quick Look makes of a PDF.
    static let sampleThumbnail: CGImage? = {
        let side = 56
        guard
            let context = CGContext(
                data: nil, width: side, height: side, bitsPerComponent: 8, bytesPerRow: 0,
                space: CGColorSpaceCreateDeviceRGB(),
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)
        else { return nil }
        context.setFillColor(CGColor(gray: 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: side, height: side))
        context.setFillColor(CGColor(gray: 0.35, alpha: 1))
        context.fill(CGRect(x: 8, y: 40, width: 24, height: 6))
        context.setFillColor(CGColor(gray: 0.7, alpha: 1))
        for line in 0..<5 {
            context.fill(CGRect(x: 8, y: 30 - line * 6, width: line == 4 ? 20 : 40, height: 3))
        }
        return context.makeImage()
    }()
}
