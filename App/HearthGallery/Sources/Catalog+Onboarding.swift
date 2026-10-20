// The sign-in and integration panels onboarding is built from.

import HearthUI

extension Catalog {
    static var onboarding: [GalleryPage] {
        let package = "HearthUI"
        return [
            GalleryPage(
                package: package, name: "SignInPanel",
                states: GalleryState.each(SignInPanel.PreviewState.self, width: 480) {
                    SignInPanel(previewState: $0)
                }),
            GalleryPage(
                package: package, name: "IntegrationsPanel",
                states: GalleryState.each(IntegrationsPanel.PreviewState.self, width: 480) {
                    IntegrationsPanel(previewState: $0)
                }),
        ]
    }
}
