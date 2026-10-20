// NotesSection is the inspector's first section: what the bot remembers, a
// NoteRow each with the pinned ones first, and "3 of 40" trailing the header.
// With nothing remembered it is one quiet line. A failed call shows its text
// under the rows in the negative tone.
// See docs/model.md#knowledge and docs/design.md#empty-states-and-first-run

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Notes section.
struct NotesSection: View {
    let store: NotesStore

    var body: some View {
        InspectorSection(title: "Notes", detail: store.loaded ? store.countText : "") {
            if store.notes.isEmpty && store.loaded {
                Text("Nothing remembered yet.")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            ForEach(store.notes, id: \.id) { note in
                NoteRow(
                    text: note.text, date: note.createdAt.date, isPinned: note.pinned,
                    onPin: { pinned in Task { await store.pin(note, pinned) } },
                    onEdit: { text in Task { await store.edit(note, text: text) } },
                    onDelete: { Task { await store.delete(note) } })
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.negative))
            }
        }
    }
}
