// The native service with a fake extractor, then the live server in-process:
// a scan drawn here, written to a temporary file, and posted to the listener
// the way the core posts it.

import CoreGraphics
import CoreText
import Foundation
import HearthKit
import PDFKit
import Synchronization
import Testing

@testable import HearthNative

private let token = "fedcba9876543210"

/// Answers with canned pages and records what it was asked.
private final class FakeExtractor: TextExtracting {
    let asked = Mutex<[(String, [Int])]>([])
    let pages: [Hearth_V1_ExtractedPage]
    let error: (any Error)?

    init(pages: [Hearth_V1_ExtractedPage] = [], error: (any Error)? = nil) {
        self.pages = pages
        self.error = error
    }

    func extract(path: String, pages: [Int], until end: Deadline) throws -> ExtractedPages {
        asked.withLock { $0.append((path, pages)) }
        if let error { throw error }
        return self.pages
    }
}

private func post(_ route: NativeRoute, _ body: String, token: String = token) -> NativeParse {
    let head = [
        "POST \(route.rawValue) HTTP/1.1", "Content-Type: application/json",
        "Authorization: Bearer \(token)", "Content-Length: \(body.utf8.count)",
    ]
    return NativeHTTP.parse(Data((head.joined(separator: "\r\n") + "\r\n\r\n" + body).utf8))
}

/// The status line's code and the body of a reply.
private func split(_ reply: Data) -> (Int, Data) {
    let text = String(bytes: reply, encoding: .utf8) ?? ""
    let status = Int(text.split(separator: " ")[1]) ?? 0
    let body = text.components(separatedBy: "\r\n\r\n").dropFirst().joined(separator: "\r\n\r\n")
    return (status, Data(body.utf8))
}

private func page(_ number: Int32, _ text: String) -> Hearth_V1_ExtractedPage {
    var page = Hearth_V1_ExtractedPage()
    page.page = number
    page.text = text
    page.source = .recognised
    return page
}

@Test func servesPingAndExtract() async throws {
    let fake = FakeExtractor(pages: [page(3, "Seen")])
    let service = NativeService(token: token, extractor: fake)
    let (pingStatus, pingBody) = split(await service.respond(to: post(.ping, "{}")))
    #expect(pingStatus == 200)
    #expect(try Hearth_V1_NativeServicePingResponse(jsonUTF8Data: pingBody).version == 1)

    let (status, body) = split(
        await service.respond(to: post(.extractText, #"{"path":"/a/scan.pdf","pages":[3,4]}"#)))
    #expect(status == 200)
    #expect(try Hearth_V1_ExtractTextResponse(jsonUTF8Data: body).pages == [page(3, "Seen")])
    #expect(fake.asked.withLock { $0.map(\.0) } == ["/a/scan.pdf"])
    #expect(fake.asked.withLock { $0.map(\.1) } == [[3, 4]])
}

@Test func refusesBeforeTheExtractor() async {
    let fake = FakeExtractor()
    let service = NativeService(token: token, extractor: fake)
    let eleven = #"{"path":"/a.pdf","pages":[1,2,3,4,5,6,7,8,9,10,11]}"#
    #expect(split(await service.respond(to: post(.extractText, eleven))).0 == 400)
    #expect(split(await service.respond(to: post(.extractText, #"{"pages":[0]}"#))).0 == 400)
    #expect(split(await service.respond(to: post(.extractText, "not json"))).0 == 400)
    #expect(split(await service.respond(to: post(.extractText, "{}", token: "wrong"))).0 == 401)
    #expect(fake.asked.withLock { $0.isEmpty })
}

@Test func saysWhyItRefusedAFile() async throws {
    let service = NativeService(
        token: token, extractor: FakeExtractor(error: ExtractRefusal("the file is over 64 MB")))
    let (status, body) = split(await service.respond(to: post(.extractText, #"{"path":"/a"}"#)))
    #expect(status == 400)
    let json = try JSONSerialization.jsonObject(with: body) as? [String: String]
    #expect(json?["message"] == "the file is over 64 MB")
}

@Test func dropsPagesUntilTheReplyFits() async throws {
    let big = String(repeating: "x", count: 1024 * 1024)
    let pages = (1...6).map { page(Int32($0), big) }
    let service = NativeService(token: token, extractor: FakeExtractor(pages: pages))
    let reply = await service.respond(to: post(.extractText, #"{"path":"/a"}"#))
    #expect(reply.count <= NativeService.maxReply + 200)
    let decoded = try Hearth_V1_ExtractTextResponse(jsonUTF8Data: split(reply).1)
    #expect(decoded.pages.map(\.page) == [1, 2, 3])
}

@Test func refusesAPathItShouldNotRead() throws {
    for path in [
        "relative.pdf", "/a/../b.pdf", "/a//b.pdf", "/a/./b.pdf", "/nonexistent/x.pdf", "/tmp",
    ] {
        #expect(throws: ExtractRefusal.self) { try TextExtractor.read(path) }
    }
    let dir = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: dir) }
    let file = dir.appending(path: "a.txt")
    try Data("x".utf8).write(to: file)
    let link = dir.appending(path: "link.txt")
    try FileManager.default.createSymbolicLink(at: link, withDestinationURL: file)
    #expect(throws: ExtractRefusal.self) { try TextExtractor.read(link.path) }
    #expect(try TextExtractor.read(file.path) == Data("x".utf8))
}

@Test func aPDFWithNoPagesReadsNothing() {
    let empty = PDFDocument()
    #expect(empty.pageCount == 0)
    let far = ContinuousClock.now + .seconds(30)
    #expect(TextExtractor.pdfPages(empty, [1, 3], far).isEmpty)
    #expect(TextExtractor.pdfPages(empty, [], far).isEmpty)
}

/// Never returns until the test lets it go.
private final class StuckExtractor: TextExtracting {
    let gate = DispatchSemaphore(value: 0)
    let calls = Mutex(0)

    func extract(path: String, pages: [Int], until end: Deadline) throws -> ExtractedPages {
        calls.withLock { $0 += 1 }
        gate.wait()
        return [page(1, "late")]
    }
}

@Test func aReadThatNeverEndsIsAbandonedAndTheNextIsBusy() async throws {
    let stuck = StuckExtractor()
    let service = NativeService(token: token, extractor: stuck, replyAfter: .milliseconds(200))
    let started = ContinuousClock.now
    let (late, _) = split(await service.respond(to: post(.extractText, #"{"path":"/a"}"#)))
    #expect(late == 504)
    #expect(started.duration(to: .now) < .seconds(5))
    let (busy, body) = split(await service.respond(to: post(.extractText, #"{"path":"/a"}"#)))
    #expect(busy == 503)
    let json = try JSONSerialization.jsonObject(with: body) as? [String: String]
    #expect(json?["code"] == "unavailable")
    #expect(stuck.calls.withLock { $0 } == 1)

    stuck.gate.signal()
    var status = 0
    for _ in 0..<50 where status != 200 {
        try await Task.sleep(for: .milliseconds(20))
        stuck.gate.signal()
        status = split(await service.respond(to: post(.extractText, #"{"path":"/a"}"#))).0
    }
    #expect(status == 200)
}

/// A one-page PDF whose page is only a picture of `text`: a scan.
private func scanPDF(_ text: String, at url: URL) throws {
    let width = 1200
    let height = 300
    let bitmap = try #require(
        CGContext(
            data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
            space: CGColorSpaceCreateDeviceGray(), bitmapInfo: CGImageAlphaInfo.none.rawValue))
    bitmap.setFillColor(CGColor(gray: 1, alpha: 1))
    bitmap.fill(CGRect(x: 0, y: 0, width: width, height: height))
    let font = CTFontCreateWithName("Helvetica-Bold" as CFString, 96, nil)
    let attributes = [kCTFontAttributeName: font] as CFDictionary
    let line = CTLineCreateWithAttributedString(
        CFAttributedStringCreate(nil, text as CFString, attributes))
    bitmap.textPosition = CGPoint(x: 40, y: 110)
    CTLineDraw(line, bitmap)
    let image = try #require(bitmap.makeImage())
    var box = CGRect(x: 0, y: 0, width: 600, height: 150)
    let pdf = try #require(CGContext(url as CFURL, mediaBox: &box, nil))
    pdf.beginPDFPage(nil)
    pdf.draw(image, in: box)
    pdf.endPDFPage()
    pdf.closePDF()
}

/// Posts `body` to the live server at `port` with `bearer`.
private func send(_ body: String, to port: UInt16, bearer: String) async throws -> (Int, Data) {
    let url = try #require(
        URL(string: "http://127.0.0.1:\(port)\(NativeRoute.extractText.rawValue)"))
    var request = URLRequest(url: url)
    request.httpMethod = "POST"
    request.setValue("application/json", forHTTPHeaderField: "Content-Type")
    request.setValue("Bearer \(bearer)", forHTTPHeaderField: "Authorization")
    request.httpBody = Data(body.utf8)
    let (data, response) = try await URLSession.shared.data(for: request)
    return ((response as? HTTPURLResponse)?.statusCode ?? 0, data)
}

@Test func readsAScanOverTheLiveServer() async throws {
    let dir = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: dir) }
    let scan = dir.appending(path: "scan.pdf").resolvingSymlinksInPath()
    try scanPDF("HEARTH READS SCANS", at: scan)

    let server = try NativeServer(token: token, extractor: TextExtractor())
    let port = try await server.start()
    defer { server.stop() }
    #expect(port > 0)

    let body = #"{"path":"\#(scan.path)","pages":[1]}"#
    #expect(try await send(body, to: port, bearer: "wrong").0 == 401)
    let (status, data) = try await send(body, to: port, bearer: token)
    #expect(status == 200)
    let pages = try Hearth_V1_ExtractTextResponse(jsonUTF8Data: data).pages
    #expect(pages.map(\.page) == [1])
    #expect(pages.first?.source == .recognised)
    #expect(pages.first?.text.uppercased().contains("HEARTH") == true)
}
