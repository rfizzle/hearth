// Shaped deliverables: the whole deliverable with every widget as written,
// live and with its measures missing, a file from a newer Hearth, and each
// shipped template's sample; then each widget in each of its states; the chip
// in the transcript and the window.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var deliverables: [GalleryPage] {
        let package = "HearthUI"
        let tile: CGFloat = 260
        let wide: CGFloat = 520
        return [
            GalleryPage(
                package: package, name: "DeliverableChip",
                states: GalleryState.each(DeliverableChip.PreviewState.self, width: 360) {
                    DeliverableChip(previewState: $0)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "DeliverableScreen",
                states: GalleryState.each(DeliverableScreen.PreviewState.self, width: 720) {
                    DeliverableScreen(previewState: $0).frame(height: 560)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "SaveTemplateSheet",
                states: [
                    GalleryState(
                        name: "empty", width: Theme.Size.sheetWidth,
                        view: AnyView(
                            SaveTemplateSheet(
                                store: TemplatesStore(client: PreviewClient()),
                                specPath: "/none.deliverable.json", title: "Office this week"
                            ) {})),
                    GalleryState(
                        name: "refused", width: Theme.Size.sheetWidth,
                        view: AnyView(
                            SaveTemplateSheet(
                                store: TemplatesStore(
                                    client: PreviewClient(),
                                    error: "report is a shipped template; pick another id"),
                                specPath: "/none.deliverable.json", title: "Report"
                            ) {})),
                ]),
            GalleryPage(
                package: package, name: "DeliverableView",
                states: GalleryState.each(DeliverableView.PreviewState.self, width: 580) {
                    DeliverableView(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "MetricTile",
                states: GalleryState.each(MetricTile.PreviewState.self, width: tile) {
                    MetricTile(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "ChartCard",
                states: GalleryState.each(ChartCard.PreviewState.self, width: wide) {
                    ChartCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "TableCard",
                states: GalleryState.each(TableCard.PreviewState.self, width: wide) {
                    TableCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "ListCard",
                states: GalleryState.each(ListCard.PreviewState.self, width: tile + 100) {
                    ListCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "ProgressCard",
                states: GalleryState.each(ProgressCard.PreviewState.self, width: tile) {
                    ProgressCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "LightCard",
                states: GalleryState.each(LightCard.PreviewState.self, width: tile) {
                    LightCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "CalloutCard",
                states: GalleryState.each(CalloutCard.PreviewState.self, width: tile + 100) {
                    CalloutCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "TextBlock",
                states: GalleryState.each(TextBlock.PreviewState.self, width: wide) {
                    TextBlock(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "UnsupportedCard",
                states: GalleryState.each(UnsupportedCard.PreviewState.self, width: tile + 100) {
                    UnsupportedCard(previewState: $0)
                }),
        ]
    }
}
