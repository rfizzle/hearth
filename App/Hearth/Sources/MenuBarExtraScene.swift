// MenuBarExtraScene is hearth in the menu bar: a flame that says, at a glance,
// whether anyone is working and whether anything waits on the owner, and a
// panel (MenuBarPanel, the system's menu-extra window) with the quick ask, who
// is working and what needs the owner. Settings › General turns it off. The
// extra counts as the app being open, so closing the window leaves hearth
// running; SwiftUI already keeps an app with closed windows running and the
// activation policy stays regular. A status item can carry no badge, so the
// count lives in the panel's "Needs you" header and in the item's spoken
// name, never beside the flame.
//
// QuickAskScene is the window the global shortcut opens, since nothing opens
// the extra's panel from code: the quick ask alone, floating at the top of the
// screen like Spotlight, never opened at launch or restored, closed by Escape.
// See docs/design.md#window-anatomy and docs/product.md

import AppKit
import HearthDesign
import HearthFeatures
import SwiftUI

/// The menu bar extra.
struct MenuBarExtraScene: Scene {
    let model: AppModel
    /// Whether the extra is in the menu bar; Settings › General's toggle.
    @AppStorage(MenuBarExtraScene.shownKey) private var isShown = true

    /// Where Settings keeps the toggle.
    static let shownKey = "menuBarExtra"

    var body: some Scene {
        MenuBarExtra(isInserted: $isShown) {
            MenuBarContent(model: model)
        } label: {
            MenuBarLabel(model: model)
        }
        .menuBarExtraStyle(.window)
    }
}

/// The flame in the menu bar, named for VoiceOver by what it shows.
private struct MenuBarLabel: View {
    let model: AppModel
    @Environment(\.openWindow) private var openWindow

    var body: some View {
        let store = model.session?.menuBar
        Image(nsImage: StatusGlyph.image(for: store?.glyph ?? .rest))
            .accessibilityLabel(store?.spokenName ?? "Hearth")
            .onAppear { model.windowOpener = openWindow }
    }
}

/// The panel, over the session when there is one.
private struct MenuBarContent: View {
    let model: AppModel
    @Environment(\.openWindow) private var openWindow
    @Environment(\.openSettings) private var openSettings

    var body: some View {
        Group {
            if let session = model.session {
                MenuBarPanel(
                    store: session.menuBar, orgName: session.org?.name ?? "Your team",
                    onOpenConversation: { model.showMain(at: $0, openWindow: openWindow) },
                    onSettings: {
                        openSettings()
                        NSApp.activate()
                    })
            } else {
                Text(model.supervisor.status)
                    .font(Theme.callout)
                    .foregroundStyle(.secondary)
                    .padding(Theme.Spacing.md)
                    .frame(width: Theme.Size.menuExtraWidth, alignment: .leading)
            }
        }
        .onAppear { model.windowOpener = openWindow }
    }
}

/// The quick ask's own window, for the global shortcut.
struct QuickAskScene: Scene {
    let model: AppModel

    var body: some Scene {
        Window("Quick Ask", id: AppModel.quickAskWindowID) {
            QuickAskContent(model: model)
        }
        .windowStyle(.hiddenTitleBar)
        .windowLevel(.floating)
        .windowResizability(.contentSize)
        .defaultPosition(.top)
        .defaultLaunchBehavior(.suppressed)
        .restorationBehavior(.disabled)
    }
}

/// The quick ask over the session, or what the helper is doing.
private struct QuickAskContent: View {
    let model: AppModel
    @Environment(\.openWindow) private var openWindow
    @Environment(\.dismissWindow) private var dismissWindow

    var body: some View {
        Group {
            if let session = model.session {
                QuickAskPanel(store: session.menuBar) { conversationID in
                    model.showMain(at: conversationID, openWindow: openWindow)
                    dismissWindow(id: AppModel.quickAskWindowID)
                }
            } else {
                Text(model.supervisor.status)
                    .font(Theme.callout)
                    .foregroundStyle(.secondary)
                    .padding(Theme.Spacing.md)
                    .frame(width: Theme.Size.menuExtraWidth, alignment: .leading)
            }
        }
        .onExitCommand { dismissWindow(id: AppModel.quickAskWindowID) }
        .onAppear { model.windowOpener = openWindow }
    }
}

/// The status item's image: a template flame per state, drawn once each.
/// Rest is the outline; someone working fills it; something waiting adds a
/// small dot at the top trailing corner, cut out of the flame by a clear ring
/// so it reads at menu bar size. The menu bar tints a template image for its
/// appearance, so it is drawn in one colour.
@MainActor
enum StatusGlyph {
    /// The status item's size and the dot's, in points.
    static let size: CGFloat = 18
    static let dot: CGFloat = 6
    static let ring: CGFloat = 1

    private static var drawn: [MenuBarStore.Glyph: NSImage] = [:]

    /// The image for `glyph`, drawn the first time it is asked for.
    static func image(for glyph: MenuBarStore.Glyph) -> NSImage {
        if let image = drawn[glyph] { return image }
        let renderer = ImageRenderer(content: GlyphView(glyph: glyph))
        renderer.scale = NSScreen.main?.backingScaleFactor ?? 2
        let image =
            renderer.nsImage
            ?? NSImage(systemSymbolName: "flame", accessibilityDescription: nil)
            ?? NSImage()
        image.isTemplate = true
        drawn[glyph] = image
        return image
    }

    private struct GlyphView: View {
        let glyph: MenuBarStore.Glyph

        var body: some View {
            ZStack(alignment: .topTrailing) {
                Image(systemName: glyph.working ? "flame.fill" : "flame")
                    .resizable()
                    .scaledToFit()
                    .frame(width: StatusGlyph.size, height: StatusGlyph.size)
                if glyph.needsYou {
                    ZStack {
                        Circle()
                            .frame(
                                width: StatusGlyph.dot + 2 * StatusGlyph.ring,
                                height: StatusGlyph.dot + 2 * StatusGlyph.ring
                            )
                            .blendMode(.destinationOut)
                        Circle().frame(width: StatusGlyph.dot, height: StatusGlyph.dot)
                    }
                }
            }
            .compositingGroup()
            .foregroundStyle(.black)
            .frame(width: StatusGlyph.size, height: StatusGlyph.size)
        }
    }
}

extension MenuBarStore.Glyph {
    /// Nobody working and nothing waiting.
    static let rest = MenuBarStore.Glyph(working: false, needsYou: false)
}
