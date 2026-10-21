// DeliverableChip is a shaped deliverable in the transcript: the spec file, the
// page and the PDF the core wrote for it, drawn as one bordered chip like a
// FileRow with the dashboard symbol and the deliverable's own title (the
// file's stem when the spec does not decode). Clicking it, or Return when
// focused, opens the deliverable's window; Space opens Quick Look on the PDF
// when there is one. The context menu offers Open, Quick Look PDF, Open in
// Browser and Show in Finder. It is a focusable view with the button trait,
// not a Button, since a Button takes Space for itself. The page is only ever
// handed to the browser; the app never loads it.
// See docs/design.md#the-transcript, docs/decisions.md (D45)

import HearthDesign
import HearthKit
import QuickLook
import SwiftUI

/// A deliverable in the transcript.
public struct DeliverableChip: View {
    /// The id of the window group that shows a deliverable; its value is the
    /// spec file's path.
    public static let windowID = "deliverable"

    let group: DeliverableGroup
    /// Whether the title was given, so the file is not read for it.
    private let titleIsGiven: Bool

    @Environment(\.openWindow) private var openWindow
    @Environment(\.revealInFinder) private var revealInFinder
    @Environment(\.openInBrowser) private var openInBrowser
    @State private var title: String?
    @State private var previewURL: URL?

    init(group: DeliverableGroup, title: String? = nil) {
        self.group = group
        self.titleIsGiven = title != nil
        _title = State(initialValue: title)
    }

    private var name: String { title ?? group.stem }

    public var body: some View {
        chip
            .onTapGesture { open() }
            .focusable()
            .onKeyPress(.return) {
                open()
                return .handled
            }
            .onKeyPress(.space) {
                guard group.pdf != nil else { return .ignored }
                quickLook()
                return .handled
            }
            .quickLookPreview($previewURL)
            .help(group.pdf == nil ? "Open" : "Open, or Quick Look the PDF (Space)")
            .accessibilityElement(children: .ignore)
            .accessibilityAddTraits(.isButton)
            .accessibilityLabel("\(name), open")
            .accessibilityAction { open() }
            .accessibilityAction(named: "Quick Look PDF") { quickLook() }
            .contextMenu { menu }
            .task(id: group.spec.path) {
                guard !titleIsGiven else { return }
                title = await Self.title(atPath: group.spec.path)
            }
    }

    private var chip: some View {
        HStack(spacing: Theme.Spacing.xs) {
            Image(systemName: "chart.bar.doc.horizontal")
                .foregroundStyle(Theme.color(.textSecondary))
                .accessibilityHidden(true)
            Text(name)
                .font(Theme.callout)
                .foregroundStyle(Theme.color(.textPrimary))
                .lineLimit(1)
                .truncationMode(.middle)
        }
        .padding(.horizontal, Theme.Spacing.sm)
        .padding(.vertical, Theme.Spacing.xs)
        .overlay(
            RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous)
                .stroke(.separator)
        )
        .contentShape(RoundedRectangle(cornerRadius: Theme.Radius.sm, style: .continuous))
    }

    @ViewBuilder private var menu: some View {
        Button("Open") { open() }
        if group.pdf != nil {
            Button("Quick Look PDF") { quickLook() }
        }
        if let html = group.html {
            Button("Open in Browser") { openInBrowser(html.path) }
        }
        Button("Show in Finder") { revealInFinder(group.spec.path) }
    }

    private func open() {
        openWindow(id: Self.windowID, value: group.spec.path)
    }

    private func quickLook() {
        guard let pdf = group.pdf else { return }
        previewURL = URL(fileURLWithPath: pdf.path)
    }

    /// The title the spec file at `path` gives itself, or nil when it cannot
    /// be read, does not decode or has no title.
    nonisolated static func title(atPath path: String) async -> String? {
        await Task.detached {
            guard let data = FileManager.default.contents(atPath: path),
                let spec = try? Deliverable(json: data), !spec.title.isEmpty
            else { return nil }
            return spec.title
        }.value
    }
}

extension DeliverableChip {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case threeFiles = "three files"
        case loneSpec = "a lone spec"
        case undecodable

        private static let folder = Fixtures.bot.folderPath + "/"

        var group: DeliverableGroup {
            func file(_ name: String) -> Hearth_V1_FileRef {
                .with {
                    $0.name = name
                    $0.path = Self.folder + name
                }
            }
            switch self {
            case .threeFiles:
                return DeliverableGroup(
                    spec: file("Launch dashboard.deliverable.json"),
                    html: file("Launch dashboard.html"),
                    pdf: file("Launch dashboard.pdf"))
            case .loneSpec:
                return DeliverableGroup(
                    spec: file("Weekly digest.deliverable.json"), html: nil, pdf: nil)
            case .undecodable:
                return DeliverableGroup(
                    spec: file("march-figures.deliverable.json"), html: nil, pdf: nil)
            }
        }

        var title: String? {
            switch self {
            case .threeFiles: "Launch dashboard"
            case .loneSpec: "Weekly digest"
            case .undecodable: nil
            }
        }
    }

    /// A chip showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(group: state.group, title: state.title)
    }
}

#Preview("Three files") { DeliverableChip(previewState: .threeFiles).padding() }
#Preview("A lone spec") { DeliverableChip(previewState: .loneSpec).padding() }
#Preview("Undecodable") { DeliverableChip(previewState: .undecodable).padding() }
