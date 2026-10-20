// InspectorView is the column beside a conversation: a list of collapsible
// sections about the selected bot, each its own feature store and view,
// composed in order: Notes, then the Handbook. A section is a header row with the
// title and, trailing, a quiet count or action, over its rows; the header
// folds the section away. The system gives the column its material.
// See docs/design.md#window-anatomy

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// What the selected bot remembers, and later what it follows and spends.
public struct InspectorView: View {
    let notes: NotesStore
    let handbook: HandbookStore

    /// The inspector over the stores for one bot. A new section adds its store
    /// here and its view to the list below, after the sections before it.
    public init(notes: NotesStore, handbook: HandbookStore) {
        self.notes = notes
        self.handbook = handbook
    }

    public var body: some View {
        List {
            NotesSection(store: notes)
            HandbookSection(store: handbook)
        }
        .listStyle(.inset)
        .scrollContentBackground(.hidden)
        .task(id: notes.botID) {
            async let notesLoaded: Void = notes.load()
            async let handbookLoaded: Void = handbook.load()
            _ = await (notesLoaded, handbookLoaded)
        }
    }
}

/// One collapsible section of the inspector: a title with a trailing caption
/// over its rows.
struct InspectorSection<Content: View>: View {
    let title: String
    let detail: String
    @ViewBuilder let content: () -> Content

    @State private var isExpanded = true

    var body: some View {
        DisclosureGroup(isExpanded: $isExpanded) {
            content()
        } label: {
            HStack {
                Text(title).font(Theme.headline)
                Spacer(minLength: Theme.Spacing.sm)
                Text(detail)
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.textSecondary))
                    .monospacedDigit()
            }
            .accessibilityElement(children: .combine)
        }
    }
}
