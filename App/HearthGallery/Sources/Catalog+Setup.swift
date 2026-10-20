// Setup: the concern row with a suggestion, without one and at its longest,
// and the guided project sheet at each step: the opener, the opener without a
// model, a question ("done when"), the card with the draft's concerns and with
// them settled, the form, the form editing a project, and the opener with no
// Chief of Staff. Each sheet runs its own store over PreviewClient and keeps
// its form preference under a key of its own, so the gallery never changes
// the app's. The Motion page's two setup rows are here too.

import HearthDesign
import HearthFeatures
import HearthKit
import HearthUI
import SwiftUI

extension Catalog {
    static var setup: [GalleryPage] {
        [
            GalleryPage(
                package: "HearthUI", name: "ConcernRow",
                states: GalleryState.each(ConcernRow.PreviewState.self, width: 480) {
                    ConcernRow(previewState: $0)
                }),
            GalleryPage(
                package: "HearthFeatures", name: "SetupSheet",
                states: [
                    setupSheet("opener", height: 560) { _ in },
                    setupSheet("opener in fallback", canDraft: false, height: 560) {
                        $0.paragraph = paragraph
                    },
                    setupSheet("asking \u{201C}done when\u{201D}", canDraft: false, height: 520) {
                        $0.paragraph = paragraph
                        await $0.draft()
                        $0.drafts["name", default: WidgetDraft()].text = "Launch the newsletter"
                        await $0.next()
                    },
                    setupSheet("card with concerns", height: 940) {
                        $0.paragraph = paragraph
                        await $0.draft()
                    },
                    setupSheet("card without", height: 760) {
                        $0.paragraph = paragraph
                        await $0.draft()
                        for concern in $0.concerns { $0.keep(concern) }
                    },
                    setupSheet("form", height: 860) {
                        $0.paragraph = paragraph
                        await $0.draft()
                        $0.showForm()
                    },
                    setupSheet("editing", editing: Fixtures.projects[0], height: 860) { _ in },
                    setupSheet("no Chief of Staff", chief: nil, height: 560) { _ in },
                ]),
        ]
    }

    private static let paragraph =
        "I want to start a newsletter for the studio. About 400 people are on the old mailing "
        + "list and we have no template. First issue before the end of November."

    /// A sheet whose store `prepare` walks to the state shown.
    private static func setupSheet(
        _ name: String, canDraft: Bool = true, editing: Hearth_V1_Project? = nil,
        chief: Hearth_V1_Bot? = Fixtures.chiefOfStaff, height: CGFloat,
        prepare: @escaping @MainActor (SetupStore) async -> Void
    ) -> GalleryState {
        let store = setupStore(canDraft: canDraft, editing: editing, key: name)
        Task { await prepare(store) }
        return GalleryState(
            name: name, width: Theme.Size.sheetWidth,
            view: AnyView(
                SetupSheet(store: store, chief: chief, onDone: { _ in }, onCancel: {})
                    .frame(height: height)))
    }

    private static func setupStore(
        canDraft: Bool = true, editing: Hearth_V1_Project? = nil, key: String
    ) -> SetupStore {
        let client = PreviewClient()
        let projects = ProjectsStore(client: client, relay: EventRelay(client: client))
        let preference = "gallery.setup." + key
        UserDefaults.standard.removeObject(forKey: preference)
        return SetupStore(
            script: .project, client: client, projects: projects, canDraft: canDraft,
            editing: editing, preferenceKey: preference)
    }

    /// Draft, then Back: the step cross-fades while the column's height moves.
    static var setupStepMotion: MotionStep {
        let store = setupStore(key: "motion-step")
        store.paragraph = paragraph
        return step(
            "Setup step changes", button: "Draft or Back",
            note: "The step cross-fades while the column's height moves with it.",
            play: {
                if store.step == .opener { Task { await store.draft() } } else { store.back() }
            },
            content: {
                SetupSheet(
                    store: store, chief: Fixtures.chiefOfStaff, onDone: { _ in },
                    onCancel: {}
                ).frame(height: 900)
            })
    }

    /// Use this, then Keep mine, then a fresh draft brings both back.
    static var concernMotion: MotionStep {
        let store = setupStore(key: "motion-concern")
        store.paragraph = paragraph
        Task { await store.draft() }
        return step(
            "A concern is settled", button: "Use this, Keep mine, or draft again",
            note: "The row fades under the quick fade while the card's height moves.",
            play: {
                if let concern = store.concerns.first {
                    store.use(concern)
                } else {
                    Task { await store.draft() }
                }
            },
            content: {
                SetupSheet(
                    store: store, chief: Fixtures.chiefOfStaff, onDone: { _ in },
                    onCancel: {}
                ).frame(height: 900)
            })
    }
}
