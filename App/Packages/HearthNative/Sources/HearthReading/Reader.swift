// Reader is the reading service's one call, the only way text recognition
// reaches a file: the app sends an open file and the pages wanted, the service
// reads them with TextExtractor and replies with the pages as protobuf bytes,
// or with one of the extractor's fixed sentences when it refuses the file.
//
// The service is its own process, sandboxed with nothing granted, so the open
// file is the only thing of the owner's it can read, and a file that crashes
// PDFKit or Vision ends that process alone. It never receives a path. Each
// call reads for at most extractTime, and a page that never returns is cut
// off by the watchdog, which ends the process a little after that; the app
// does the same from its side if no reply comes.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthKit
import SwiftProtobuf

/// The service's interface over XPC. The request is a serialised
/// ExtractTextRequest with no path; the reply is a serialised
/// ExtractTextResponse, or a refusal sentence, or neither when the read failed.
@objc(HearthReadingService)
public protocol ReadingService {
    func read(
        _ file: FileHandle, request: Data, seconds: Double,
        withReply reply: @escaping @Sendable (Data?, String?) -> Void)
}

public enum Reading {
    /// The service's bundle identifier, which is also the name it is found by.
    public static let serviceName = "com.rfizzle.hearth.reader"
    /// The longest a call reads for; the service stops at the next page after.
    public static let extractTime: Duration = .seconds(25)
    /// How long after extractTime the watchdog ends a call still running.
    public static let overrunGrace: Duration = .seconds(2)
    /// The largest request taken: ten page numbers need far less.
    public static let maxRequest = 1024

    /// The interface both ends of the connection use.
    public static func interface() -> NSXPCInterface {
        NSXPCInterface(with: ReadingService.self)
    }
}

/// The exported object: reads one file per call, one call at a time.
public final class Reader: NSObject, ReadingService {
    private let overrun: @Sendable () -> Void
    private let extractor = TextExtractor()

    /// A reader whose watchdog calls `overrun` when a call runs past its time;
    /// the service ends its process there, a test records it.
    public init(overrun: @escaping @Sendable () -> Void) {
        self.overrun = overrun
    }

    public func read(
        _ file: FileHandle, request: Data, seconds: Double,
        withReply reply: @escaping @Sendable (Data?, String?) -> Void
    ) {
        let most = Double(Reading.extractTime.components.seconds)
        let budget = seconds.isFinite ? min(max(seconds, 0), most) : 0
        let grace = Double(Reading.overrunGrace.components.seconds)
        let watchdog = DispatchWorkItem(block: overrun)
        DispatchQueue.global().asyncAfter(deadline: .now() + budget + grace, execute: watchdog)
        defer { watchdog.cancel() }
        let end = ContinuousClock.now + .milliseconds(Int(budget * 1000))
        let (data, refusal) = answer(file, request, until: end)
        reply(data, refusal)
    }

    private func answer(_ file: FileHandle, _ body: Data, until end: Deadline) -> (Data?, String?) {
        guard body.count <= Reading.maxRequest,
            let request = try? Hearth_V1_ExtractTextRequest(serializedBytes: body),
            request.path.isEmpty, request.pages.count <= TextExtractor.maxPages,
            request.pages.allSatisfy({ (1...1_048_576).contains($0) })
        else { return (nil, nil) }
        do {
            let bytes = try TextExtractor.read(file)
            var response = Hearth_V1_ExtractTextResponse()
            response.pages = try extractor.extract(
                bytes, pages: request.pages.map(Int.init), until: end)
            return (try response.serializedData(), nil)
        } catch let refusal as ExtractRefusal {
            return (nil, refusal.message)
        } catch {
            return (nil, nil)
        }
    }
}
