// NativeService is the native server's answers over complete requests,
// without the network: the token check, the two routes, the JSON, the time a
// read may take and the reply's size.
//
// A read is never done here: it goes to the reading service, a sandboxed
// process of its own, which reads for at most extractTime (25 s). The call
// waits for its reply at most replyTime (28 s, inside the core's 30 s); with
// no reply by then the service is killed and the call answered late, and when
// the service dies first the call is answered as failed. A read that comes
// while one is running is answered busy. So at most one read is ever alive,
// the serial loop is never held past replyTime, and a crafted file can end
// the reading service but never the app.
// See docs/decisions.md#hearth--decisions

import Foundation
import HearthKit
import HearthReading
import SwiftProtobuf
import os

public struct NativeService: Sendable {
    /// Bumped when the native surface changes.
    public static let version = Int32(HearthNative.version)
    /// The most pages one call reads.
    public static let maxPages = TextExtractor.maxPages
    /// The time the service reads for; it stops at the next page after it.
    public static let extractTime: Duration = Reading.extractTime
    /// The time a call waits for the read before it kills the service and
    /// answers without it.
    public static let replyTime: Duration = .seconds(28)
    /// The largest reply; pages are dropped from the end until it fits.
    public static let maxReply = 4 * 1024 * 1024

    let token: String
    let reader: any PageReading
    private let replyAfter: Duration
    private let log = Logger(subsystem: "com.rfizzle.hearth", category: "native")

    /// A service that answers requests carrying `token` through `reader`,
    /// waiting `replyAfter` for a read; a test shortens it.
    public init(token: String, reader: any PageReading, replyAfter: Duration = replyTime) {
        self.token = token
        self.reader = reader
        self.replyAfter = replyAfter
    }

    /// The bytes to send back for what was received.
    public func respond(to parsed: NativeParse) async -> Data {
        switch parsed {
        case .incomplete: return NativeHTTP.reply(.malformed)
        case .refused(let error): return NativeHTTP.reply(error)
        case .request(let request):
            switch NativeHTTP.route(request, token: token) {
            case .failure(let error): return NativeHTTP.reply(error)
            case .success(.ping):
                var ping = Hearth_V1_NativeServicePingResponse()
                ping.version = Self.version
                return encoded(ping)
            case .success(.extractText): return await extract(request.body)
            }
        }
    }

    private func extract(_ body: Data) async -> Data {
        guard let request = try? Hearth_V1_ExtractTextRequest(jsonUTF8Data: body),
            request.pages.count <= Self.maxPages,
            request.pages.allSatisfy({ (1...1_048_576).contains($0) })
        else { return NativeHTTP.reply(.malformed) }
        let started = ContinuousClock.now
        let name = URL(filePath: request.path).lastPathComponent
        let outcome = await reader.read(
            path: request.path, pages: request.pages.map(Int.init), extractFor: Self.extractTime,
            waitFor: replyAfter)
        switch outcome {
        case .busy:
            log.info("refused \(name): busy with an earlier read")
            return NativeHTTP.reply(.busy)
        case .late:
            log.error("gave up on \(name) after \(started.duration(to: .now)); reader killed")
            return NativeHTTP.reply(.late)
        case .pages(let pages):
            log.info("read \(pages.count) pages of \(name) in \(started.duration(to: .now))")
            return fitted(pages)
        case .refused(let message):
            log.info("refused \(name): \(message, privacy: .public)")
            return NativeHTTP.reply(
                NativeHTTPError(status: 400, code: "invalid_argument", message: message))
        case .crashed:
            log.error("the reader ended while reading \(name)")
            return NativeHTTP.reply(.failed)
        case .failed:
            log.error("could not read \(name)")
            return NativeHTTP.reply(.failed)
        }
    }

    /// The reply for `pages`, dropping pages from the end until it is at most
    /// maxReply bytes. A page dropped is a page the core reports unread.
    private func fitted(_ pages: ExtractedPages) -> Data {
        var response = Hearth_V1_ExtractTextResponse()
        response.pages = pages
        while true {
            let data = encoded(response)
            if data.count <= Self.maxReply || response.pages.isEmpty { return data }
            response.pages.removeLast()
        }
    }

    private func encoded(_ message: some SwiftProtobuf.Message) -> Data {
        guard let body = try? message.jsonUTF8Data() else { return NativeHTTP.reply(.failed) }
        return NativeHTTP.reply(body)
    }
}
