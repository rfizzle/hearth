// HandbookSection is the inspector's second section: the pages every run of
// this bot reads, a PageRow each, "New Page…" trailing under them and the
// context meter at the foot. Clicking a page, or New Page, opens the editor
// sheet. With no pages it is one quiet line. A failed call shows its text
// under the rows in the negative tone.
// See docs/model.md#knowledge and docs/design.md#empty-states-and-first-run

import HearthDesign
import HearthKit
import HearthUI
import SwiftUI

/// The Handbook section.
struct HandbookSection: View {
    let store: HandbookStore

    @State private var editing: Editing?

    /// One presentation of the sheet: a page, or a new one.
    private struct Editing: Identifiable {
        let page: Hearth_V1_HandbookPage?
        var id: String { page?.id ?? "new" }
    }

    var body: some View {
        InspectorSection(
            title: "Handbook", detail: store.loaded ? "\(store.pages.count)" : ""
        ) {
            if store.pages.isEmpty && store.loaded {
                Text("No pages yet.")
                    .font(Theme.callout)
                    .foregroundStyle(Theme.color(.textSecondary))
            }
            ForEach(store.pages, id: \.id) { page in
                PageRow(
                    title: page.title, scope: store.scopeText(page), bytes: page.bytes,
                    onOpen: { editing = Editing(page: page) },
                    onDelete: { Task { await store.delete(page) } })
            }
            HStack {
                Spacer()
                Button("New Page…") { editing = Editing(page: nil) }
            }
            if store.loaded {
                ContextMeter(used: store.meter.usedBytes, cap: store.meter.capBytes)
            }
            if let error = store.error {
                Label(error, systemImage: "exclamationmark.triangle")
                    .font(Theme.caption)
                    .foregroundStyle(Theme.color(.negative))
            }
        }
        .sheet(item: $editing) { editing in
            HandbookPageSheet(store: store, page: editing.page, onDone: { self.editing = nil })
        }
    }
}
