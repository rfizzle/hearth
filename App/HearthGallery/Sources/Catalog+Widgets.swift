// The controls a bot can ask the owner to answer with, and the card that
// asks: every kind of control empty, valid, invalid and answered, and the
// prompt card in words, for each kind, through its grace and answered.

import HearthUI
import SwiftUI

extension Catalog {
    static var widgets: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "WidgetControl",
                states: GalleryState.each(WidgetControl.PreviewState.self, width: 480) {
                    WidgetControl(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "PromptCard",
                states: GalleryState.each(PromptCard.PreviewState.self, width: 600) {
                    PromptCard(previewState: $0)
                }),
        ]
    }
}
