// GalleryPage is one entry in the sidebar: a unit or the screens, grouped
// under the package it lives in. A GalleryState is one card on that page.

import SwiftUI

/// One state of a unit, drawn once per appearance by StateCard.
struct GalleryState: Identifiable {
    let name: String
    /// The width the unit is shown at; nil lets it take its natural size.
    let width: CGFloat?
    let view: AnyView

    var id: String { name }
}

extension GalleryState {
    /// One state for every case of `previewState`. A unit without a
    /// `PreviewState` has nothing to pass here, so the build says so.
    @MainActor
    static func each<State: CaseIterable & RawRepresentable<String>, Content: View>(
        _ previewState: State.Type, width: CGFloat? = nil,
        @ViewBuilder _ make: (State) -> Content
    ) -> [GalleryState] {
        previewState.allCases.map { state in
            GalleryState(name: state.rawValue, width: width, view: AnyView(make(state)))
        }
    }
}

/// A sidebar page: the package it belongs to, its name and its cards.
struct GalleryPage: Identifiable {
    let package: String
    let name: String
    let states: [GalleryState]

    var id: String { name }
}
