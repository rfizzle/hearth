// The design primitives, each in every state.

import HearthDesign
import SwiftUI

extension Catalog {
    static var design: [GalleryPage] {
        let package = "HearthDesign"
        return [
            GalleryPage(
                package: package, name: "Avatar",
                states: GalleryState.each(Avatar.PreviewState.self) { Avatar(previewState: $0) }),
            GalleryPage(
                package: package, name: "StatusDot",
                states: GalleryState.each(StatusDot.PreviewState.self) {
                    StatusDot(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "Pill",
                states: GalleryState.each(Pill.PreviewState.self) { Pill(previewState: $0) }),
            GalleryPage(
                package: package, name: "CardSurface",
                states: GalleryState.each(CardSurface<Text>.PreviewState.self, width: 240) {
                    CardSurface(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "DisclosureRow",
                states: GalleryState.each(DisclosureRow<Text, Text>.PreviewState.self, width: 560) {
                    DisclosureDemo(state: $0)
                }),
            GalleryPage(
                package: package, name: "FileIcon",
                states: GalleryState.each(FileIcon.PreviewState.self) { FileIcon(previewState: $0) }
            ),
        ]
    }
}

/// A disclosure row that opens on click; the unit's own preview init holds
/// its binding fixed.
private struct DisclosureDemo: View {
    @State private var isExpanded: Bool

    init(state: DisclosureRow<Text, Text>.PreviewState) {
        _isExpanded = State(initialValue: state.isExpanded)
    }

    var body: some View {
        DisclosureRow(
            isExpanded: $isExpanded,
            label: { Text("Searched the web") },
            content: { Text("3 results for “quarterly filing dates”") })
    }
}
