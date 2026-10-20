// What the inspector shows beside a conversation: the note row in each state
// the usage summary and its budget field, and the inspector itself with notes
// and with none.

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
                package: "HearthUI", name: "PageRow",
                states: GalleryState.each(PageRow.PreviewState.self, width: 300) {
                    PageRow(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthUI", name: "ContextMeter",
                states: GalleryState.each(ContextMeter.PreviewState.self, width: 300) {
                    ContextMeter(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthUI", name: "UsageSummary",
                states: GalleryState.each(UsageSummary.PreviewState.self, width: 300) {
                    UsageSummary(previewState: $0).padding(.horizontal, Theme.Spacing.lg)
                }),
            GalleryPage(
                package: "HearthUI", name: "BudgetField",
                states: GalleryState.each(BudgetField.PreviewState.self, width: 280) {
                    BudgetField(previewState: $0)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "HandbookPageSheet",
                states: handbookSheetStates()),
            GalleryPage(
                package: "HearthFeatures", name: "Inspector",
                states: [inspectorWithNotes(), inspectorEmpty()]),
        ]
    }

    /// The handbook store for the Dana fixture, or for a bot with no pages.
    static func handbookStore(botID: String = Fixtures.bot.id) -> HandbookStore {
        let client = PreviewClient()
        return HandbookStore(
            client: client, relay: EventRelay(client: client), botID: botID,
            botName: Fixtures.bot.name)
    }

    /// The usage store for the Dana fixture: October, Dana at 92% of her budget.
    static func usageStore() -> UsageStore {
        let client = PreviewClient()
        return UsageStore(client: client, relay: EventRelay(client: client))
    }

    /// PreviewClient's five notes and two pages, the first note pinned.
    static func inspectorWithNotes() -> GalleryState {
        let client = PreviewClient()
        let store = NotesStore(
            client: client, relay: EventRelay(client: client), botID: Fixtures.bot.id)
        return GalleryState(
            name: "notes", width: Theme.Size.inspectorIdeal,
            view: AnyView(
                InspectorView(
                    notes: store, handbook: handbookStore(), usage: usageStore(),
                    botName: Fixtures.bot.name
                )
                .frame(height: 760)))
    }

    /// A bot with nothing remembered and no pages.
    static func inspectorEmpty() -> GalleryState {
        let client = PreviewClient()
        let store = NotesStore(
            client: client, relay: EventRelay(client: client), botID: "nobody")
        return GalleryState(
            name: "empty", width: Theme.Size.inspectorIdeal,
            view: AnyView(
                InspectorView(
                    notes: store, handbook: handbookStore(botID: "nobody"),
                    usage: usageStore(), botName: "Nobody"
                )
                .frame(height: 420)))
    }

    /// A new page, an existing page, and a save the core refuses.
    static func handbookSheetStates() -> [GalleryState] {
        [
            GalleryState(
                name: "new", width: Theme.Size.sheetWidth,
                view: AnyView(HandbookPageSheet(store: handbookStore(), onDone: {}))),
            GalleryState(
                name: "edit", width: Theme.Size.sheetWidth,
                view: AnyView(
                    HandbookPageSheet(
                        store: handbookStore(), page: Fixtures.handbookPages[1], onDone: {}))),
            GalleryState(
                name: "refused", width: Theme.Size.sheetWidth,
                view: AnyView(RefusedHandbookSheet(store: handbookStore()))),
        ]
    }
}

/// The sheet after a Save the core refused: the store is asked to save a page
/// with no title, as Save would, and the core's words show under the editor.
private struct RefusedHandbookSheet: View {
    let store: HandbookStore

    var body: some View {
        HandbookPageSheet(store: store, onDone: {})
            .task {
                await store.put(
                    title: "", markdown: "Write plainly.", scope: .with { $0.everyone = true })
            }
    }
}
