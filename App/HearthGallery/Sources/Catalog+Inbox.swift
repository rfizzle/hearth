// What waits on the owner: the decision bar in its wide and narrow layouts,
// and the approval, question and failure cards in each state.

import HearthDesign
import HearthUI
import SwiftUI

extension Catalog {
    static var inbox: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "DecisionBar",
                states: DecisionBar.PreviewState.allCases.map { state in
                    GalleryState(
                        name: state.rawValue, width: state.width,
                        view: AnyView(DecisionBar(previewState: state)))
                }),
            GalleryPage(
                package: package, name: "ApprovalCard",
                states: GalleryState.each(ApprovalCard.PreviewState.self, width: 600) {
                    ApprovalCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "QuestionCard",
                states: GalleryState.each(QuestionCard.PreviewState.self, width: 600) {
                    QuestionCard(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "FailureCard",
                states: GalleryState.each(FailureCard.PreviewState.self, width: 600) {
                    FailureCard(previewState: $0)
                }),
        ]
    }
}
