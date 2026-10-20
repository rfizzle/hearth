// GalleryApp is the review surface for the look: a sidebar of pages grouped
// by package and, for the page chosen, one card per state with the light and
// dark canvases side by side. Launch arguments make a capture scriptable:
// --page <Name>, --text largest, --window-width and --window-height <points>,
// and --play, which plays each change on the Motion page in turn.

import HearthDesign
import SwiftUI

/// What the launch arguments ask for.
struct LaunchOptions {
    var page: String?
    var largestText = false
    var play = false
    var windowWidth: CGFloat = 1560
    var windowHeight: CGFloat = 900

    init(_ arguments: [String]) {
        for (index, argument) in arguments.enumerated() {
            let value = arguments.indices.contains(index + 1) ? arguments[index + 1] : nil
            switch argument {
            case "--page": page = value
            case "--text": largestText = value == "largest"
            case "--play": play = true
            case "--window-width":
                windowWidth = value.flatMap(Double.init).map { CGFloat($0) } ?? windowWidth
            case "--window-height":
                windowHeight = value.flatMap(Double.init).map { CGFloat($0) } ?? windowHeight
            default: break
            }
        }
    }
}

@main
struct GalleryApp: App {
    private let options = LaunchOptions(ProcessInfo.processInfo.arguments)

    var body: some Scene {
        WindowGroup("Hearth Gallery") {
            GalleryRoot(options: options)
        }
        .defaultSize(width: options.windowWidth, height: options.windowHeight)
    }
}

private struct GalleryRoot: View {
    @State private var selection: String?
    @State private var largestText: Bool
    private let play: Bool

    private let pages = Catalog.pages

    init(options: LaunchOptions) {
        let wanted = options.page ?? (options.play ? "Motion" : nil)
        let named = Catalog.pages.first { $0.name.lowercased() == wanted?.lowercased() }
        _selection = State(initialValue: (named ?? Catalog.pages.first)?.name)
        _largestText = State(initialValue: options.largestText)
        play = options.play
    }

    private var page: GalleryPage? {
        pages.first { $0.name == selection }
    }

    private var packages: [String] {
        var seen: [String] = []
        for page in pages where !seen.contains(page.package) { seen.append(page.package) }
        return seen
    }

    var body: some View {
        NavigationSplitView {
            List(selection: $selection) {
                ForEach(packages, id: \.self) { package in
                    Section(package) {
                        ForEach(pages.filter { $0.package == package }) { page in
                            Label(page.name, systemImage: Self.symbol(for: package))
                                .badge(page.states.count)
                                .tag(page.name)
                        }
                    }
                }
            }
            .navigationSplitViewColumnWidth(
                min: Theme.Size.sidebarMin, ideal: Theme.Size.sidebarIdeal,
                max: Theme.Size.sidebarMax)
        } detail: {
            if let page {
                ScrollViewReader { proxy in
                    ScrollView {
                        LazyVStack(alignment: .leading, spacing: Theme.Spacing.xl) {
                            ForEach(page.states) { StateCard(state: $0).id($0.id) }
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                    }
                    .task(id: page.name) {
                        if play && page.name == Catalog.motionPage {
                            await Catalog.playMotion { proxy.scrollTo($0, anchor: .center) }
                        }
                    }
                }
                .contentMargins(Theme.Spacing.xl)
                .dynamicTypeSize(largestText ? .accessibility5 : .large)
                .navigationTitle(page.name)
                .navigationSubtitle(page.package)
            } else {
                ContentUnavailableView("Choose a page", systemImage: "sidebar.left")
            }
        }
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Picker("Text size", selection: $largestText) {
                    Text("Default").tag(false)
                    Text("Largest").tag(true)
                }
                .pickerStyle(.segmented)
            }
        }
    }

    private static func symbol(for package: String) -> String {
        switch package {
        case "HearthDesign": "paintpalette"
        case "HearthUI": "rectangle.3.group"
        default: "macwindow"
        }
    }
}
