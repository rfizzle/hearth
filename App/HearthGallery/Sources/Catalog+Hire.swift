// The pieces of the hire sheet.

import HearthUI

extension Catalog {
    static var hire: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "TemplateCard",
                states: GalleryState.each(TemplateCard.PreviewState.self, width: 240) {
                    TemplateCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "TierPicker",
                states: GalleryState.each(TierPicker.PreviewState.self, width: 480) {
                    TierPicker(previewState: $0)
                }),
        ]
    }
}
