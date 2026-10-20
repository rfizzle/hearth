// TextBlock is a deliverable's prose on a card: Markdown with headings, lists,
// tables and code, drawn by swift-markdown-ui with a theme made from the
// design's text colours, the system body size and the person's accent for
// links. The text is model output, so the renderer is held in: images are not
// loaded (both image providers draw nothing; the core refuses images anyway),
// raw HTML is never interpreted, and a link opens in the system browser or
// mail only when it is http, https or mailto. The callout uses the same
// renderer. This is the one file that imports MarkdownUI.
// See docs/decisions.md (D47, D48)

import AppKit
import HearthDesign
import HearthKit
import MarkdownUI
import SwiftUI

/// Markdown prose from a deliverable.
public struct TextBlock: View {
    let markdown: String

    public init(markdown: String) {
        self.markdown = markdown
    }

    public var body: some View {
        DeliverableMarkdown(markdown: markdown)
            .deliverableCard()
    }
}

/// Markdown drawn the one way every deliverable widget draws it.
struct DeliverableMarkdown: View {
    let markdown: String

    var body: some View {
        Markdown(markdown)
            .markdownTheme(Self.theme)
            .markdownImageProvider(NoImages())
            .markdownInlineImageProvider(NoInlineImages())
            .environment(
                \.openURL,
                OpenURLAction { url in DeliverableDisplay.opens(url) ? .systemAction : .discarded }
            )
            .textSelection(.enabled)
    }

    /// The theme: the basic one in the design's colours at the system's body
    /// size, headings a step up and semibold, code monospaced.
    static var theme: MarkdownUI.Theme {
        let body = NSFont.preferredFont(forTextStyle: .body).pointSize
        let separator = Color(nsColor: .separatorColor)
        return MarkdownUI.Theme.basic
            .text {
                ForegroundColor(HearthDesign.Theme.color(.textPrimary))
                FontSize(body)
            }
            .link { ForegroundColor(.accentColor) }
            .code {
                FontFamilyVariant(.monospaced)
                FontSize(.em(0.92))
            }
            .heading1 { heading($0, scale: 1.3) }
            .heading2 { heading($0, scale: 1.2) }
            .heading3 { heading($0, scale: 1.1) }
            .blockquote { configuration in
                configuration.label
                    .markdownTextStyle { ForegroundColor(HearthDesign.Theme.color(.textSecondary)) }
                    .padding(.leading, HearthDesign.Theme.Spacing.md)
                    .overlay(alignment: .leading) { Rectangle().fill(separator).frame(width: 2) }
            }
            .table { configuration in
                configuration.label
                    .markdownTableBorderStyle(.init(color: separator))
                    .markdownMargin(top: 0, bottom: HearthDesign.Theme.Spacing.md)
            }
            .tableCell { configuration in
                configuration.label
                    .markdownTextStyle {
                        if configuration.row == 0 { FontWeight(.semibold) }
                    }
                    .padding(.vertical, HearthDesign.Theme.Spacing.xs)
                    .padding(.horizontal, HearthDesign.Theme.Spacing.sm)
            }
    }

    private static func heading(_ configuration: BlockConfiguration, scale: Double) -> some View {
        configuration.label
            .markdownMargin(
                top: HearthDesign.Theme.Spacing.md, bottom: HearthDesign.Theme.Spacing.sm
            )
            .markdownTextStyle {
                FontWeight(.semibold)
                FontSize(.em(scale))
            }
    }
}

/// Draws no block image: a deliverable carries none, and nothing it names is
/// fetched.
private struct NoImages: ImageProvider {
    func makeImage(url: URL?) -> some View { EmptyView() }
}

/// Loads no inline image, for the same reason.
private struct NoInlineImages: InlineImageProvider {
    struct NotLoaded: Error {}

    func image(with url: URL, label: String) async throws -> Image { throw NotLoaded() }
}

extension TextBlock {
    /// The states the gallery and previews show.
    public enum PreviewState: String, CaseIterable, Sendable {
        case rich = "headings, lists, a table, a link"
        case plain = "a paragraph"
        case refused = "an image and a script link, not drawn"

        var markdown: String {
            switch self {
            case .rich:
                guard case .text(let text) = Fixtures.kitchenSink.sections[3].widgets[1].content
                else {
                    preconditionFailure("the kitchen sink ends with a text block")
                }
                return text + "\n```\nmake ci\n```\n"
            case .plain:
                guard
                    case .text(let text) = Fixtures.deliverable("report").sections[0].widgets[0]
                        .content
                else { preconditionFailure("the report opens with a text block") }
                return text
            case .refused:
                return
                    "Before ![a chart](https://example.com/chart.png) after, "
                    + "and [a script](javascript:alert(1)) that does not open."
            }
        }
    }

    /// A block showing `state`, for the gallery and previews.
    public init(previewState state: PreviewState) {
        self.init(markdown: state.markdown)
    }
}

#Preview("Rich") { TextBlock(previewState: .rich).frame(width: 480).padding() }
