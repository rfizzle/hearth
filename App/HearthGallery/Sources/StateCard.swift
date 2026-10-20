// StateCard shows one state: a headline, then the light and dark canvases
// side by side. Each canvas is the real window background for its appearance
// at the state's width or its natural size, so the pair is how the unit looks
// in either mode without a toggle.

import HearthDesign
import SwiftUI

struct StateCard: View {
    let state: GalleryState

    var body: some View {
        VStack(alignment: .leading, spacing: Theme.Spacing.md) {
            Text(state.name)
                .font(Theme.headline)
                .accessibilityAddTraits(.isHeader)
            // Side by side when the window is wide enough, stacked when not,
            // so a canvas is never cut off.
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: Theme.Spacing.lg) {
                    canvas(.light)
                    canvas(.dark)
                }
                VStack(alignment: .leading, spacing: Theme.Spacing.lg) {
                    canvas(.light)
                    canvas(.dark)
                }
            }
        }
    }

    private func canvas(_ scheme: ColorScheme) -> some View {
        let shape = RoundedRectangle(cornerRadius: Theme.Radius.md)
        return state.view
            .frame(width: state.width)
            .fixedSize(horizontal: state.width == nil, vertical: true)
            .padding(Theme.Spacing.lg)
            .background(.background)
            .clipShape(shape)
            .overlay(shape.stroke(.separator))
            .environment(\.colorScheme, scheme)
            .accessibilityLabel("\(state.name), \(scheme == .light ? "light" : "dark")")
    }
}
