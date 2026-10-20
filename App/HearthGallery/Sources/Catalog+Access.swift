// What a bot can see and do: the capability row and the folder row.

import HearthDesign
import HearthUI

extension Catalog {
    static var access: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "AccessRow",
                states: GalleryState.each(AccessRow.PreviewState.self, width: 560) {
                    AccessRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: package, name: "FolderRow",
                states: GalleryState.each(FolderRow.PreviewState.self, width: 560) {
                    FolderRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
        ]
    }
}
