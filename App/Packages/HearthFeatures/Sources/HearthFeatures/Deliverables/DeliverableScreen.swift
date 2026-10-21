// DeliverableScreen is a shaped deliverable in its own window: the title and
// the cards DeliverableView draws from the spec file, scrolling. The toolbar
// has Show in Finder, Open PDF (Quick Look), Open in Browser when those files
// are beside the spec, and a menu with Save as Template and Reload. The file is read when the
// window opens and again each time it becomes the key window; a bound widget
// is live (the subtitle says so) while the window has the session's client. A
// file that does not decode is one plain sentence. Nothing here loads the page: Open in
// Browser hands it to the system. A chart opened large is an overlay over the
// whole window.
// See docs/design.md#window-anatomy, docs/decisions.md (D45)

import HearthDesign
import HearthKit
import HearthUI
import QuickLook
import SwiftUI

/// The window for one deliverable.
public struct DeliverableScreen: View {
    @State private var store: DeliverableStore

    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.openInBrowser) private var openInBrowser
    @Environment(\.controlActiveState) private var activeState
    @State private var previewURL: URL?
    @State private var templates: TemplatesStore?
    @State private var isSavingTemplate = false
    @State private var expanded: ExpandedTarget?
    @Namespace private var zoom
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    /// The chart open large, and the card it zoomed from.
    private struct ExpandedTarget {
        let id: UUID
        let shape: ChartCard.Shape
    }

    /// A window for the spec file at `path`; given the session's `client` and
    /// `relay` its bound widgets are live.
    public init(path: String, client: (any HearthClient)? = nil, relay: EventRelay? = nil) {
        _store = State(initialValue: DeliverableStore(path: path, client: client, relay: relay))
        _templates = State(initialValue: client.map { TemplatesStore(client: $0) })
    }

    /// A window over `store`, for the gallery and previews.
    public init(store: DeliverableStore, templates: TemplatesStore? = nil) {
        _store = State(initialValue: store)
        _templates = State(initialValue: templates)
    }

    public var body: some View {
        content
            .frame(minWidth: 480, minHeight: 320)
            .navigationTitle(store.title)
            .navigationSubtitle(store.isLive ? "Live" : "")
            .toolbar { toolbar }
            .quickLookPreview($previewURL)
            .environment(
                \.chartZoom,
                ChartZoom(namespace: zoom) { id, shape in
                    setExpanded(ExpandedTarget(id: id, shape: shape))
                }
            )
            .overlay {
                if let expanded {
                    ExpandedChart(id: expanded.id, shape: expanded.shape, namespace: zoom) {
                        setExpanded(nil)
                    }
                }
            }
            .sheet(isPresented: $isSavingTemplate) {
                if let templates {
                    SaveTemplateSheet(
                        store: templates, specPath: store.path, title: store.title
                    ) { isSavingTemplate = false }
                }
            }
            .task { await store.load() }
            .onChange(of: activeState) { _, state in
                if state == .key { Task { await store.load() } }
            }
    }

    private func setExpanded(_ target: ExpandedTarget?) {
        withAnimation(ExpandedChart.animation(reduceMotion: reduceMotion)) { expanded = target }
    }

    @ViewBuilder private var content: some View {
        switch store.state {
        case .loading:
            ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
        case .shown(let spec):
            ScrollView {
                DeliverableView(spec: spec, values: store.values.isEmpty ? nil : store.values)
                    .padding(Theme.Spacing.xl)
            }
        case .unreadable:
            ContentUnavailableView(
                "This file isn't a deliverable Hearth can show",
                systemImage: "doc.questionmark",
                description: Text("It may have been changed, or made by a newer Hearth."))
        }
    }

    @ToolbarContentBuilder private var toolbar: some ToolbarContent {
        ToolbarItemGroup {
            Button("Show in Finder", systemImage: "folder") { revealInFinder(store.path) }
            if store.hasPDF {
                Button("Open PDF", systemImage: "doc.richtext") {
                    previewURL = URL(fileURLWithPath: store.pdfPath)
                }
            }
            if store.hasPage {
                Button("Open in Browser", systemImage: "safari") {
                    openInBrowser(store.pagePath)
                }
            }
            Menu("More", systemImage: "ellipsis.circle") {
                if templates != nil, case .shown = store.state {
                    Button("Save as Template…") {
                        templates?.clearError()
                        isSavingTemplate = true
                    }
                }
                Button("Reload") { Task { await store.load() } }
            }
        }
    }
}

extension DeliverableScreen {
    /// The states the gallery shows.
    public enum PreviewState: String, CaseIterable, Sendable {
        case dashboard
        case live
        case undecodable
    }

    /// A window showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        let path = Fixtures.bot.folderPath + "/Launch dashboard.deliverable.json"
        switch state {
        case .dashboard:
            self.init(
                store: DeliverableStore(
                    path: path, previewing: Fixtures.kitchenSink, hasPage: true, hasPDF: true))
        case .live:
            self.init(
                store: DeliverableStore(
                    path: path, previewing: Fixtures.kitchenSink, hasPage: true, hasPDF: true,
                    values: Fixtures.liveMeasures))
        case .undecodable:
            self.init(store: DeliverableStore(path: path))
        }
    }
}
