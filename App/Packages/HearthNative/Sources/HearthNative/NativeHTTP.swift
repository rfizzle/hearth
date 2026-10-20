// NativeHTTP is the whole of HTTP the native server speaks: one request
// shape, a POST of at most 64 KB with a Content-Length, parsed by hand, and a
// reply that closes the connection. Anything else is refused before a body is
// read: no chunked bodies, no folded or repeated headers that matter, no
// header section over 8 KB, no other HTTP version. The caller is the core on
// the same Mac, holding the launch token; everyone else gets a fixed error.
// Errors are Connect's JSON shape, so the core's generated client reads them.
// See docs/decisions.md#hearth--decisions

import Foundation

/// The two routes the native server answers.
public enum NativeRoute: String, Sendable, CaseIterable {
    case ping = "/hearth.v1.NativeService/Ping"
    case extractText = "/hearth.v1.NativeService/ExtractText"
}

/// A request whose header section and body have arrived in full.
public struct NativeRequest: Sendable, Equatable {
    public var method: String
    public var target: String
    /// The Authorization header, or nil when there was none.
    public var authorization: String?
    public var contentType: String?
    public var body: Data
}

/// A refusal, as an HTTP status and a Connect error. The message is one of
/// the fixed sentences below; nothing from the request is echoed in it.
public struct NativeHTTPError: Error, Sendable, Equatable {
    public let status: Int
    public let code: String
    public let message: String

    static let malformed = Self(status: 400, code: "invalid_argument", message: "malformed request")
    static let headTooLarge = Self(
        status: 413, code: "resource_exhausted", message: "the request headers are too large")
    static let bodyTooLarge = Self(
        status: 413, code: "resource_exhausted", message: "the request body is too large")
    static let unauthenticated = Self(
        status: 401, code: "unauthenticated", message: "the launch token does not match")
    static let notPost = Self(status: 405, code: "unimplemented", message: "only POST is served")
    static let notFound = Self(status: 404, code: "unimplemented", message: "no such procedure")
    static let notJSON = Self(
        status: 415, code: "invalid_argument", message: "the body must be application/json")
    static let failed = Self(status: 500, code: "internal", message: "the file could not be read")
    static let busy = Self(
        status: 503, code: "unavailable", message: "busy with an earlier read")
    static let late = Self(
        status: 504, code: "deadline_exceeded", message: "the read took too long")
}

/// What a buffer of received bytes holds so far.
public enum NativeParse: Sendable, Equatable {
    /// More bytes are needed.
    case incomplete
    case request(NativeRequest)
    case refused(NativeHTTPError)
}

public enum NativeHTTP {
    /// The most bytes of request line and headers, with the blank line.
    public static let maxHead = 8 * 1024
    /// The most bytes of body.
    public static let maxBody = 64 * 1024
    /// The most header lines.
    public static let maxHeaders = 64

    private static let headEnd = Data("\r\n\r\n".utf8)

    /// Parses `buffer`, the bytes received on one connection so far. Bytes
    /// past the declared body are ignored: one request a connection.
    public static func parse(_ buffer: Data) -> NativeParse {
        let window = buffer.prefix(maxHead)
        guard let end = window.firstRange(of: headEnd) else {
            return buffer.count >= maxHead ? .refused(.headTooLarge) : .incomplete
        }
        let head = buffer[buffer.startIndex..<end.lowerBound]
        guard
            head.allSatisfy({ $0 == 0x0D || $0 == 0x0A || $0 == 0x09 || (0x20...0x7E).contains($0) }
            ),
            let text = String(data: head, encoding: .ascii)
        else { return .refused(.malformed) }
        let parsed: (NativeRequest, Int)
        switch requestHead(text) {
        case .failure(let error): return .refused(error)
        case .success(let value): parsed = value
        }
        var (request, length) = parsed
        let bodyStart = end.upperBound
        guard buffer.endIndex - bodyStart >= length else { return .incomplete }
        request.body = Data(buffer[bodyStart..<bodyStart + length])
        return .request(request)
    }

    /// The request line and headers, and the body's length.
    private typealias Head = Result<(NativeRequest, Int), NativeHTTPError>

    private static func requestHead(_ text: String) -> Head {
        let lines = text.components(separatedBy: "\r\n")
        let start = lines[0].split(separator: " ", omittingEmptySubsequences: false)
        guard start.count == 3, start[2] == "HTTP/1.1", !start[0].isEmpty, start[1].hasPrefix("/"),
            lines.count - 1 <= maxHeaders
        else { return .failure(.malformed) }
        var request = NativeRequest(
            method: String(start[0]), target: String(start[1]), authorization: nil,
            contentType: nil, body: Data())
        var length: Int?
        for line in lines.dropFirst() {
            guard let (name, value) = header(line) else { return .failure(.malformed) }
            if let error = take(name, value, into: &request, length: &length) {
                return .failure(error)
            }
        }
        guard let length else { return .failure(.malformed) }
        guard length <= maxBody else { return .failure(.bodyTooLarge) }
        return .success((request, length))
    }

    /// Takes one header into `request` or `length`, or the refusal it earns:
    /// a second Content-Length, Authorization or Content-Type, any
    /// Transfer-Encoding, or a Content-Encoding other than identity.
    private static func take(
        _ name: String, _ value: String, into request: inout NativeRequest, length: inout Int?
    ) -> NativeHTTPError? {
        switch name {
        case "content-length":
            guard length == nil, let declared = contentLength(value) else { return .malformed }
            length = declared
        case "authorization":
            guard request.authorization == nil else { return .malformed }
            request.authorization = value
        case "content-type":
            guard request.contentType == nil else { return .malformed }
            request.contentType = value
        case "transfer-encoding": return .malformed
        case "content-encoding" where value.lowercased() != "identity": return .notJSON
        default: break
        }
        return nil
    }

    /// A header line's lower-cased name and trimmed value; nil for a line
    /// that is not `name: value`, including a folded continuation.
    private static func header(_ line: String) -> (String, String)? {
        guard let colon = line.firstIndex(of: ":") else { return nil }
        let name = line[..<colon]
        let token = CharacterSet.alphanumerics.union(CharacterSet(charactersIn: "!#$%&'*+-.^_`|~"))
        guard !name.isEmpty, name.unicodeScalars.allSatisfy(token.contains) else { return nil }
        let value = line[line.index(after: colon)...].trimmingCharacters(
            in: .init(charactersIn: " \t"))
        return (name.lowercased(), value)
    }

    /// A Content-Length of digits only, at most seven of them.
    private static func contentLength(_ value: String) -> Int? {
        guard (1...7).contains(value.count), value.allSatisfy({ $0.isASCII && $0.isNumber }) else {
            return nil
        }
        return Int(value)
    }

    /// The route `request` may take: the token first, so a caller without it
    /// learns nothing of the routes, then POST, then a known route, then JSON.
    /// A header section parse refuses (malformed, or a body over maxBody) is
    /// refused before this, from the headers alone and with nothing read.
    public static func route(_ request: NativeRequest, token: String) -> Result<
        NativeRoute, NativeHTTPError
    > {
        guard authorized(request.authorization, token: token) else {
            return .failure(.unauthenticated)
        }
        guard request.method == "POST" else { return .failure(.notPost) }
        guard let route = NativeRoute(rawValue: request.target) else { return .failure(.notFound) }
        let type = request.contentType?.split(separator: ";").first?.trimmingCharacters(
            in: .whitespaces)
        guard type?.lowercased() == "application/json" else { return .failure(.notJSON) }
        return .success(route)
    }

    /// Whether `header` is `Bearer <token>`, compared in constant time: every
    /// byte of the expected value is looked at whatever the header holds, and
    /// the differences are folded together rather than returned at the first.
    /// An empty token matches nothing.
    public static func authorized(_ header: String?, token: String) -> Bool {
        guard !token.isEmpty, let header else { return false }
        let given = Array(header.utf8)
        let want = Array(("Bearer " + token).utf8)
        var difference: UInt8 = given.count == want.count ? 0 : 1
        for index in want.indices {
            difference |= want[index] ^ (index < given.count ? given[index] : 0)
        }
        return difference == 0
    }

    /// A 200 reply carrying `body`, JSON.
    public static func reply(_ body: Data) -> Data {
        message(status: 200, body: body)
    }

    /// The reply for `error`: its status and Connect's JSON error body.
    public static func reply(_ error: NativeHTTPError) -> Data {
        let body =
            (try? JSONEncoder().encode(["code": error.code, "message": error.message])) ?? Data()
        return message(status: error.status, body: body)
    }

    private static func message(status: Int, body: Data) -> Data {
        let reasons = [
            200: "OK", 400: "Bad Request", 401: "Unauthorized", 404: "Not Found",
            405: "Method Not Allowed", 413: "Content Too Large", 415: "Unsupported Media Type",
            500: "Internal Server Error", 503: "Service Unavailable", 504: "Gateway Timeout",
        ]
        var head = "HTTP/1.1 \(status) \(reasons[status] ?? "Error")\r\n"
        head += "Content-Type: application/json\r\n"
        head += "Content-Length: \(body.count)\r\n"
        if status == 405 { head += "Allow: POST\r\n" }
        head += "Connection: close\r\n\r\n"
        return Data(head.utf8) + body
    }
}
