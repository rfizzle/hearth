// TextExtractor reads the text of pages the core could not: a PDF page's text
// through PDFKit, and when that is not text either, the page drawn at most
// 2,000 pixels on its long side and read by Vision's accurate recogniser; an
// image's text the same way, with what it shows as at most five labels Vision
// is at least half sure of. It reads the file and writes nothing.
//
// The path comes from the core, which has already checked it against the
// bot's grants. This side opens it once, refusing a link as the last part, and
// checks the open file itself: a regular file of at most 64 MB. It reads that
// file into memory and parses only those bytes, so a file swapped in after the
// check is never the one read. A picture over 50 megapixels is refused from
// its header, before any pixel is decoded.
// See docs/decisions.md#hearth--decisions

import CoreGraphics
import Foundation
import HearthKit
import ImageIO
import PDFKit
import Vision

public struct TextExtractor: TextExtracting {
    /// The largest file read, as the core's own cap.
    public static let maxFileBytes = 64 * 1024 * 1024
    /// The largest picture looked at, judged from its header.
    public static let maxPixels = 50_000_000
    /// The long side a PDF page is drawn at for recognition.
    public static let pageSide = 2000
    /// The long side an image is decoded at for recognition: larger than a
    /// page's, so a screenshot's small type stays legible.
    public static let imageSide = 4096
    /// The most bytes of text kept for one page.
    public static let maxPageText = 1024 * 1024
    /// Labels below this confidence are left out, and at most maxLabels kept.
    public static let labelConfidence: Float = 0.5
    public static let maxLabels = 5

    public init() {}

    public func extract(path: String, pages: [Int], until end: Deadline) throws -> ExtractedPages {
        let data = try Self.read(path)
        if data.prefix(5) == Data("%PDF-".utf8) {
            guard let document = PDFDocument(data: data) else {
                throw ExtractRefusal("the file could not be read as a PDF")
            }
            guard !document.isLocked else { throw ExtractRefusal("the PDF is password-protected") }
            return Self.pdfPages(document, pages, end)
        }
        guard pages.isEmpty || pages == [1] else { return [] }
        return [try Self.imagePage(data)]
    }

    /// The bytes of the file at `path`: an absolute, normalised path whose
    /// last part is not a link, opened once and checked as opened to be a
    /// regular file of at most maxFileBytes; a refusal otherwise.
    static func read(_ path: String) throws -> Data {
        let refused = ExtractRefusal("the path is not an absolute regular file Hearth can read")
        guard path.hasPrefix("/"), !path.contains("\0"), !path.contains("//"),
            !path.split(separator: "/").contains(where: { $0 == "." || $0 == ".." })
        else { throw refused }
        // O_NOFOLLOW refuses a link; O_NONBLOCK keeps a pipe from holding the
        // open, and fstat then refuses it as not a regular file.
        let descriptor = open(path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
        guard descriptor >= 0 else { throw refused }
        let handle = FileHandle(fileDescriptor: descriptor, closeOnDealloc: true)
        var status = stat()
        guard fstat(descriptor, &status) == 0, status.st_mode & S_IFMT == S_IFREG else {
            throw refused
        }
        let tooLarge = ExtractRefusal("the file is over 64 MB")
        guard status.st_size <= maxFileBytes else { throw tooLarge }
        let data = try handle.read(upToCount: maxFileBytes + 1) ?? Data()
        guard data.count <= maxFileBytes else { throw tooLarge }
        return data
    }

    /// The text of `pages` of `doc`, read until `end`. A page outside the
    /// document is left out; a document with no pages has none to read.
    static func pdfPages(_ doc: PDFDocument, _ pages: [Int], _ end: Deadline) -> ExtractedPages {
        let count = doc.pageCount
        guard count > 0 else { return [] }
        let wanted = pages.isEmpty ? Array(1...min(count, NativeService.maxPages)) : pages
        var out: ExtractedPages = []
        for number in wanted.prefix(NativeService.maxPages) {
            guard ContinuousClock.now < end else { break }
            guard number >= 1, number <= count, let page = doc.page(at: number - 1) else {
                continue
            }
            var result = Hearth_V1_ExtractedPage()
            result.page = Int32(number)
            if let text = page.string, isText(text) {
                result.text = capped(text)
                result.source = .textLayer
            } else if let image = draw(page) {
                guard let seen = try? recognise(image) else { continue }
                result.text = capped(seen.text)
                result.source = .recognised
            } else {
                continue
            }
            out.append(result)
        }
        return out
    }

    static func imagePage(_ data: Data) throws -> Hearth_V1_ExtractedPage {
        let options = [kCGImageSourceShouldCache: false] as CFDictionary
        guard let source = CGImageSourceCreateWithData(data as CFData, options),
            CGImageSourceGetCount(source) > 0,
            let properties = CGImageSourceCopyPropertiesAtIndex(source, 0, options)
                as? [CFString: Any],
            let width = properties[kCGImagePropertyPixelWidth] as? Int,
            let height = properties[kCGImagePropertyPixelHeight] as? Int, width > 0, height > 0
        else { throw ExtractRefusal("the file could not be read as a picture or a PDF") }
        let (pixels, overflow) = width.multipliedReportingOverflow(by: height)
        guard !overflow, pixels <= maxPixels else {
            throw ExtractRefusal("the picture is larger than 50 megapixels")
        }
        let thumbnail =
            [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: imageSide,
                kCGImageSourceShouldCacheImmediately: true,
            ] as CFDictionary
        guard let image = CGImageSourceCreateThumbnailAtIndex(source, 0, thumbnail) else {
            throw ExtractRefusal("the file could not be read as a picture or a PDF")
        }
        let seen = try recognise(image, labelled: true)
        var page = Hearth_V1_ExtractedPage()
        page.page = 1
        page.text = capped(seen.text)
        page.source = .recognised
        page.labels = seen.labels
        return page
    }

    /// `page` drawn on white with its long side at pageSide pixels; nil for
    /// a page with no area.
    static func draw(_ page: PDFPage) -> CGImage? {
        let box = page.bounds(for: .mediaBox)
        let long = max(box.width, box.height)
        guard box.width.isFinite, box.height.isFinite, box.width > 0, box.height > 0 else {
            return nil
        }
        let scale = CGFloat(pageSide) / long
        let width = min(pageSide, max(1, Int((box.width * scale).rounded(.up))))
        let height = min(pageSide, max(1, Int((box.height * scale).rounded(.up))))
        guard
            let context = CGContext(
                data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                space: CGColorSpaceCreateDeviceRGB(),
                bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)
        else { return nil }
        context.setFillColor(CGColor(gray: 1, alpha: 1))
        context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        context.scaleBy(x: scale, y: scale)
        context.translateBy(x: -box.minX, y: -box.minY)
        page.draw(with: .mediaBox, to: context)
        return context.makeImage()
    }

    /// The text Vision reads in `image`, line by line, and when `labelled`
    /// what it shows.
    static func recognise(_ image: CGImage, labelled: Bool = false) throws -> (
        text: String, labels: [String]
    ) {
        let text = VNRecognizeTextRequest()
        text.recognitionLevel = .accurate
        text.usesLanguageCorrection = true
        let classify = VNClassifyImageRequest()
        let handler = VNImageRequestHandler(cgImage: image)
        try handler.perform(labelled ? [text, classify] : [text])
        let lines = (text.results ?? []).compactMap { $0.topCandidates(1).first?.string }
        let labels = (classify.results ?? [])
            .filter { $0.confidence >= labelConfidence }
            .sorted { $0.confidence > $1.confidence }
            .prefix(maxLabels)
            .map(\.identifier)
        return (lines.joined(separator: "\n"), labelled ? Array(labels) : [])
    }

    /// The core's own rule, so a page is called text the same way on both
    /// sides: at least 20 characters that are not space, and no more than a
    /// tenth of them unreadable.
    static func isText(_ text: String) -> Bool {
        var solid = 0
        var bad = 0
        var total = 0
        for scalar in text.unicodeScalars {
            total += 1
            let space = scalar.properties.isWhitespace
            let control = scalar.properties.generalCategory == .control && !space
            if scalar == "\u{FFFD}" || control { bad += 1 }
            if !space { solid += 1 }
        }
        return solid >= 20 && bad * 10 <= total
    }

    /// `text` cut to maxPageText bytes.
    static func capped(_ text: String) -> String {
        guard text.utf8.count > maxPageText else { return text }
        var end = text.utf8.index(text.utf8.startIndex, offsetBy: maxPageText)
        while String.Index(end, within: text.unicodeScalars) == nil {
            end = text.utf8.index(before: end)
        }
        return String(text.unicodeScalars[..<end])
    }
}
