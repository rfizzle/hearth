// What the inspector shows beside a conversation: the note row in each state
// and the inspector itself with notes and with none.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var inspector: [GalleryPage] {
        [
            GalleryPage(
                package: "HearthUI", name: "NoteRow",
                states: GalleryState.each(NoteRow.PreviewState.self, width: 300) {
                    NoteRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "Inspector",
                states: [inspectorWithNotes(), inspectorEmpty()]),
        ]
    }

    /// PreviewClient's five notes, the first pinned.
    static func inspectorWithNotes() -> GalleryState {
        let client = PreviewClient()
        let store = NotesStore(
            client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id)
        return GalleryState(
            name: "notes", width: Theme.Size.inspectorIdeal,
            view: AnyView(InspectorView(notes: store).frame(height: 460)))
    }

    /// A bot with nothing remembered.
    static func inspectorEmpty() -> GalleryState {
        let client = PreviewClient()
        let store = NotesStore(
            client: client, relay: EventRelay(client: client), botID: "nobody")
        return GalleryState(
            name: "empty", width: Theme.Size.inspectorIdeal,
            view: AnyView(InspectorView(notes: store).frame(height: 200)))
    }
}
