// The controls a bot can ask the owner to answer with: every kind of control
// empty, valid, invalid and answered.

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
                })
        ]
    }
}
