// ChartExport takes a chart's numbers or a picture of it out of the app. The
// data is tab-separated text with a header row, which Numbers and Excel paste
// as a table; the picture is the chart drawn at twice its size, saved as a PNG
// through the system's save panel. The text is a pure function of the chart
// so it can be tested without a pasteboard.
// See docs/design.md#colour

import AppKit
import HearthKit
import SwiftUI

/// The ways a chart leaves the window.
@MainActor
enum ChartExport {
    /// The chart's series as tab-separated text: a header row, then one row
    /// per category (or slice). Values are plain numbers, not the chart's
    /// display format, so a spreadsheet can add them up; a tab or a line break
    /// in a label becomes a space so every row keeps its columns.
    nonisolated static func tabSeparated(_ shape: ChartCard.Shape) -> String {
        var rows: [[String]]
        switch shape {
        case .line(let data), .bar(let data):
            rows = [["Category"] + data.series.map(\.name)]
            for (index, category) in data.categories.enumerated() {
                rows.append(
                    [category]
                        + data.series.map { series in
                            series.values.indices.contains(index) ? plain(series.values[index]) : ""
                        })
            }
        case .pie(let pie):
            rows = [["Slice", "Value"]] + pie.slices.map { [$0.label, plain($0.value)] }
        }
        return rows.map { $0.map(clean).joined(separator: "\t") }.joined(separator: "\n") + "\n"
    }

    /// A label with tabs and line breaks replaced by spaces.
    nonisolated static func clean(_ text: String) -> String {
        String(text.map { $0 == "\t" || $0.isNewline ? " " : $0 })
    }

    /// A number written the same way in every locale: whole numbers without a
    /// fraction, the rest as the shortest text that reads back exactly.
    nonisolated static func plain(_ value: Double) -> String {
        if value.isFinite, value == value.rounded(), abs(value) < 1e15 {
            return String(Int64(value))
        }
        return String(value)
    }

    /// Puts the chart's data on the general pasteboard.
    static func copyData(_ shape: ChartCard.Shape) {
        let pasteboard = NSPasteboard.general
        pasteboard.clearContents()
        pasteboard.setString(tabSeparated(shape), forType: .string)
    }

    /// Asks where to save, then writes the chart as a PNG at 2x.
    static func saveImage(_ shape: ChartCard.Shape, title: String, scheme: ColorScheme) {
        let panel = NSSavePanel()
        panel.allowedContentTypes = [.png]
        panel.nameFieldStringValue = (title.isEmpty ? "Chart" : clean(title)) + ".png"
        panel.canCreateDirectories = true
        guard panel.runModal() == .OK, let url = panel.url else { return }
        let renderer = ImageRenderer(
            content: ChartCard(shape, mode: .image)
                .frame(width: 640, height: 400)
                .environment(\.colorScheme, scheme))
        renderer.scale = 2
        guard let image = renderer.nsImage, let tiff = image.tiffRepresentation,
            let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:])
        else { return }
        try? png.write(to: url)
    }
}

/// What a chart card tells its host: this chart wants to open large. Set by
/// the window that hosts the overlay; absent, a card has no expand button.
public struct ChartZoom {
    /// The namespace the card and the expanded view share for the zoom.
    public let namespace: Namespace.ID
    /// Opens the chart with `id` large.
    public let expand: (_ id: UUID, _ shape: ChartCard.Shape) -> Void

    public init(
        namespace: Namespace.ID, expand: @escaping (_ id: UUID, _ shape: ChartCard.Shape) -> Void
    ) {
        self.namespace = namespace
        self.expand = expand
    }
}

extension EnvironmentValues {
    @Entry public var chartZoom: ChartZoom?
}
