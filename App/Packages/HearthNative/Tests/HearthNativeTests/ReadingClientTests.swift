// The app's side of the reading service against an XPC listener in this
// process: a reader that dies mid-read, one that never replies, the real
// reader on a PDF with no pages, and what the app keeps of a reply.

import CoreGraphics
import Foundation
import HearthKit
import HearthReading
import Synchronization
import Testing

@testable import HearthNative

/// An anonymous XPC listener that exports, for its nth connection, the reader
/// `make` gives; it counts the connections, so a test sees a new service.
final class ReaderHost: NSObject, NSXPCListenerDelegate, @unchecked Sendable {
    let listener = NSXPCListener.anonymous()
    let accepted = Mutex(0)
    private let make: @Sendable (Int, NSXPCConnection) -> any ReadingService

    init(_ make: @escaping @Sendable (Int, NSXPCConnection) -> any ReadingService) {
        self.make = make
        super.init()
        listener.delegate = self
        listener.resume()
    }

    convenience init(_ make: @escaping @Sendable (Int) -> any ReadingService) {
        self.init { number, _ in make(number) }
    }

    func listener(
        _ listener: NSXPCListener, shouldAcceptNewConnection connection: NSXPCConnection
    ) -> Bool {
        let number = accepted.withLock { count in
            count += 1
            return count
        }
        connection.exportedInterface = Reading.interface()
        connection.exportedObject = make(number, connection)
        connection.resume()
        return true
    }

    func client() -> ReadingClient {
        ReadingClient { [self] in NSXPCConnection(listenerEndpoint: listener.endpoint) }
    }
}

/// Replies with canned pages, or a refusal.
private final class FixedReader: NSObject, ReadingService {
    let pages: ExtractedPages
    let refusal: String?
    init(_ pages: ExtractedPages = [], refusal: String? = nil) {
        self.pages = pages
        self.refusal = refusal
    }
    func read(
        _ file: FileHandle, request: Data, seconds: Double,
        withReply reply: @escaping @Sendable (Data?, String?) -> Void
    ) {
        if let refusal { return reply(nil, refusal) }
        var response = Hearth_V1_ExtractTextResponse()
        response.pages = pages
        reply(try? response.serializedData(), nil)
    }
}

/// Ends its connection mid-read, as a service that crashed on the file.
private final class CrashingReader: NSObject, ReadingService {
    private weak var connection: NSXPCConnection?
    init(_ connection: NSXPCConnection) { self.connection = connection }
    func read(
        _ file: FileHandle, request: Data, seconds: Double,
        withReply reply: @escaping @Sendable (Data?, String?) -> Void
    ) {
        connection?.invalidate()
    }
}

/// Never replies, as a service stuck on a page.
private final class HangingReader: NSObject, ReadingService, Sendable {
    let calls = Mutex(0)
    let held = Mutex<[@Sendable (Data?, String?) -> Void]>([])
    func read(
        _ file: FileHandle, request: Data, seconds: Double,
        withReply reply: @escaping @Sendable (Data?, String?) -> Void
    ) {
        calls.withLock { $0 += 1 }
        held.withLock { $0.append(reply) }
    }
}

private func page(_ number: Int32, _ text: String) -> Hearth_V1_ExtractedPage {
    var page = Hearth_V1_ExtractedPage()
    page.page = number
    page.text = text
    page.source = .recognised
    return page
}

/// A temporary file holding `data`, removed with its folder by `cleanup`.
private func temporaryFile(_ name: String, _ data: Data = Data("x".utf8)) throws -> (
    URL, cleanup: () -> Void
) {
    let dir = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    let file = dir.appending(path: name).resolvingSymlinksInPath()
    try data.write(to: file)
    return (file, { try? FileManager.default.removeItem(at: dir) })
}

private func read(
    _ client: ReadingClient, _ path: String, wait: Duration = .seconds(5)
) async -> ReadOutcome {
    await client.read(path: path, pages: [1], extractFor: .seconds(1), waitFor: wait)
}

@Test func aServiceThatDiesMidReadCostsThatReadAlone() async throws {
    let (file, cleanup) = try temporaryFile("a.png")
    defer { cleanup() }
    let host = ReaderHost { number, connection in
        number == 1
            ? CrashingReader(connection) as any ReadingService : FixedReader([page(1, "second")])
    }
    let client = host.client()
    #expect(await read(client, file.path) == .crashed)
    #expect(await read(client, file.path) == .pages([page(1, "second")]))
    #expect(host.accepted.withLock { $0 } == 2)
}

@Test func aServiceThatNeverRepliesIsDroppedAtTheDeadline() async throws {
    let (file, cleanup) = try temporaryFile("a.pdf")
    defer { cleanup() }
    let hanging = HangingReader()
    let host = ReaderHost { number in
        number == 1 ? hanging as any ReadingService : FixedReader([page(1, "fresh")])
    }
    let client = host.client()
    let started = ContinuousClock.now
    #expect(await read(client, file.path, wait: .milliseconds(200)) == .late)
    #expect(started.duration(to: .now) < .seconds(5))
    #expect(hanging.calls.withLock { $0 } == 1)
    #expect(await read(client, file.path) == .pages([page(1, "fresh")]))
    #expect(host.accepted.withLock { $0 } == 2)
}

@Test func aReadWhileOneRunsIsBusy() async throws {
    let (file, cleanup) = try temporaryFile("a.pdf")
    defer { cleanup() }
    let hanging = HangingReader()
    let client = ReaderHost { _ in hanging }.client()
    let first = Task { await read(client, file.path, wait: .seconds(2)) }
    for _ in 0..<100 where hanging.calls.withLock({ $0 }) == 0 {
        try await Task.sleep(for: .milliseconds(10))
    }
    #expect(await read(client, file.path) == .busy)
    #expect(await first.value == .late)
    #expect(hanging.calls.withLock { $0 } == 1)
}

@Test func theAppOpensTheFileBeforeTheServiceSeesIt() async throws {
    let host = ReaderHost { _ in FixedReader() }
    let outcome = await read(host.client(), "/nonexistent/scan.pdf")
    #expect(outcome == .refused("the path is not an absolute regular file Hearth can read"))
    #expect(host.accepted.withLock { $0 } == 0)
}

/// A well-formed PDF whose page tree has no pages, though it may claim
/// `count`. PDFKit opens no document without a page, so the service refuses
/// it as a PDF rather than reading nothing; the page guard behind that is
/// tested directly in NativeServerTests.
private func emptyPDF(claiming count: Int) -> Data {
    let objects = [
        "1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n",
        "2 0 obj << /Type /Pages /Kids [] /Count \(count) >> endobj\n",
    ]
    var text = "%PDF-1.4\n"
    var offsets: [Int] = []
    for object in objects {
        offsets.append(text.utf8.count)
        text += object
    }
    let start = text.utf8.count
    text += "xref\n0 3\n0000000000 65535 f \n"
    for offset in offsets { text += String(format: "%010d 00000 n \n", offset) }
    text += "trailer << /Size 3 /Root 1 0 R >>\nstartxref\n\(start)\n%%EOF\n"
    return Data(text.utf8)
}

@Test(arguments: [0, 3])
func aPDFWithNoPagesReadsNothingInTheService(claiming count: Int) async throws {
    let (file, cleanup) = try temporaryFile("empty.pdf", emptyPDF(claiming: count))
    defer { cleanup() }
    let host = ReaderHost { _ in Reader(overrun: {}) }
    let client = host.client()
    for pages in [[1, 3], []] {
        let outcome = await client.read(
            path: file.path, pages: pages, extractFor: .seconds(5), waitFor: .seconds(10))
        #expect(outcome == .refused("the file could not be read as a PDF"))
    }
    #expect(host.accepted.withLock { $0 } == 1)
}

@Test func keepsOnlyWhatWasAskedOfAReply() throws {
    var forged = page(1, "one")
    forged.labels = (1...9).map { "label\($0)" } + [String(repeating: "y", count: 500)]
    var unspecified = page(2, "two")
    unspecified.source = .unspecified
    var response = Hearth_V1_ExtractTextResponse()
    response.pages = [forged, page(1, "again"), unspecified, page(7, "not asked")]
    let data = try response.serializedData()
    guard case (.pages(let pages), false) = ReadingClient.outcome(data, nil, asked: [1, 2]) else {
        Issue.record("the reply was not taken as pages")
        return
    }
    #expect(pages.map(\.page) == [1])
    #expect(pages.first?.labels.count == TextExtractor.maxLabels)

    #expect(ReadingClient.outcome(nil, "words of its own", asked: [1]) == (.failed, true))
    #expect(ReadingClient.outcome(Data([0xFF, 0xFF]), nil, asked: [1]) == (.failed, true))
    let tooLarge = Data(count: ReadingClient.maxReply + 1)
    #expect(ReadingClient.outcome(tooLarge, nil, asked: [1]) == (.failed, true))
    #expect(
        ReadingClient.outcome(nil, "the file is over 64 MB", asked: [1])
            == (.refused("the file is over 64 MB"), false))
    #expect(ReadingClient.outcome(nil, nil, asked: [1]) == (.failed, false))
}
