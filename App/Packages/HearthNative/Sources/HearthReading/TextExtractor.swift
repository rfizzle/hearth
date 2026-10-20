// TextExtractor reads the text of pages the core could not: a PDF page's text
// through PDFKit, and when that is not text either, the page drawn at most
// 2,000 pixels on its long side and read by Vision's accurate recogniser; an
// image's text the same way, with what it shows as at most five labels Vision
// is at least half sure of. It reads the file and writes nothing.
//
// It runs only in the reading service, the sandboxed process the app hands a
// file to, never in the app: a crafted file then crashes or hangs that process
// alone. The path comes from the core, which has already checked it against
// the bot's grants. The app opens it once, refusing a link as the last part,
// checks the open file itself (a regular file of at most 64 MB) and passes the
// open file, never the path. The service checks it again, reads it into
// memory and parses only those bytes, so a file swapped in after the check is
// never the one read. A picture over 50 megapixels is refused from its
// header, before any pixel is decoded.
// See docs/decisions.md#hearth--decisions

import CoreGraphics
import Foundation
import HearthKit
import ImageIO
import PDFKit
import Vision

/// The pages read, and the instant reading must stop by.
public typealias ExtractedPages = [Hearth_V1_ExtractedPage]
public typealias Deadline = ContinuousClock.Instant

/// A path or file that will not be read, said in a fixed sentence.
public struct ExtractRefusal: Error, Sendable, Equatable {
    public let message: String
    public init(_ message: String) { self.message = message }

    static let notAFile = ExtractRefusal("the path is not an absolute regular file Hearth can read")
    static let tooLarge = ExtractRefusal("the file is over 64 MB")
    static let notAPDF = ExtractRefusal("the file could not be read as a PDF")
    static let locked = ExtractRefusal("the PDF is password-protected")
    static let notAPicture = ExtractRefusal("the file could not be read as a picture or a PDF")
    static let tooManyPixels = ExtractRefusal("the picture is larger than 50 megapixels")

    /// Every sentence the extractor refuses with. The app passes on only
    /// these from the service, so a service that went wrong cannot put words
    /// of its own in front of the core.
    public static let sentences: Set<String> = Set(
        [notAFile, tooLarge, notAPDF, locked, notAPicture, tooManyPixels].map(\.message))
}

public struct TextExtractor: Sendable {
    /// The most pages one call reads.
    public static let maxPages = 10
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

    /// The pages of the file whose bytes are `data` that could be read before
    /// `end`, in the order asked; `pages` are counted from 1, empty for the
    /// first pages of the file.
    public func extract(_ data: Data, pages: [Int], until end: Deadline) throws -> ExtractedPages {
        if data.prefix(5) == Data("%PDF-".utf8) {
            guard let document = PDFDocument(data: data) else {
                throw ExtractRefusal.notAPDF
            }
            guard !document.isLocked else { throw ExtractRefusal.locked }
            return Self.pdfPages(document, pages, end)
        }
        guard pages.isEmpty || pages == [1] else { return [] }
        return [try Self.imagePage(data)]
    }

    /// The file at `path` opened for reading: an absolute, normalised path
    /// whose last part is not a link, opened once and checked as opened to be
    /// a regular file of at most maxFileBytes; a refusal otherwise. Nothing of
    /// it is read here.
    public static func open(_ path: String) throws -> FileHandle {
        let refused = ExtractRefusal.notAFile
        guard path.hasPrefix("/"), !path.contains("\0"), !path.contains("//"),
            !path.split(separator: "/").contains(where: { $0 == "." || $0 == ".." })
        else { throw refused }
        // O_NOFOLLOW refuses a link; O_NONBLOCK keeps a pipe from holding the
        // open, and fstat then refuses it as not a regular file.
        let descriptor = Darwin.open(path, O_RDONLY | O_NOFOLLOW | O_NONBLOCK | O_CLOEXEC)
        guard descriptor >= 0 else { throw refused }
        let handle = FileHandle(fileDescriptor: descriptor, closeOnDealloc: true)
        _ = try size(of: handle)
        return handle
    }

    /// The bytes of the open file `handle`, checked again as a regular file of
    /// at most maxFileBytes and read from its start, at most that many.
    public static func read(_ handle: FileHandle) throws -> Data {
        _ = try size(of: handle)
        try handle.seek(toOffset: 0)
        let data = try handle.read(upToCount: maxFileBytes + 1) ?? Data()
        guard data.count <= maxFileBytes else { throw ExtractRefusal.tooLarge }
        return data
    }

    /// The size of the regular file open on `handle`; a refusal for anything
    /// else or anything over maxFileBytes.
    static func size(of handle: FileHandle) throws -> Int {
        var status = stat()
        guard fstat(handle.fileDescriptor, &status) == 0, status.st_mode & S_IFMT == S_IFREG else {
            throw ExtractRefusal.notAFile
        }
        guard status.st_size <= maxFileBytes else { throw ExtractRefusal.tooLarge }
        return Int(status.st_size)
    }

    /// The text of `pages` of `doc`, read until `end`. A page outside the
    /// document is left out; a document with no pages has none to read.
    static func pdfPages(_ doc: PDFDocument, _ pages: [Int], _ end: Deadline) -> ExtractedPages {
        let count = doc.pageCount
        guard count > 0 else { return [] }
        let wanted = pages.isEmpty ? Array(1...min(count, maxPages)) : pages
        var out: ExtractedPages = []
        for number in wanted.prefix(maxPages) {
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
        else { throw ExtractRefusal.notAPicture }
        let (pixels, overflow) = width.multipliedReportingOverflow(by: height)
        guard !overflow, pixels <= maxPixels else {
            throw ExtractRefusal.tooManyPixels
        }
        let thumbnail =
            [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceCreateThumbnailWithTransform: true,
                kCGImageSourceThumbnailMaxPixelSize: imageSide,
                kCGImageSourceShouldCacheImmediately: true,
            ] as CFDictionary
        guard let image = CGImageSourceCreateThumbnailAtIndex(source, 0, thumbnail) else {
            throw ExtractRefusal.notAPicture
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
    public static func capped(_ text: String) -> String {
        guard text.utf8.count > maxPageText else { return text }
        var end = text.utf8.index(text.utf8.startIndex, offsetBy: maxPageText)
        while String.Index(end, within: text.unicodeScalars) == nil {
            end = text.utf8.index(before: end)
        }
        return String(text.unicodeScalars[..<end])
    }
}
