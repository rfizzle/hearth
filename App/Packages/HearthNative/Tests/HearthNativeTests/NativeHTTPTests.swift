// The parser and the token check, on bytes alone: no listener.

import Foundation
import Testing

@testable import HearthNative

private let token = "0123456789abcdef"

private func request(
    _ method: String = "POST", _ target: String = NativeRoute.extractText.rawValue,
    headers: [String] = ["Content-Type: application/json", "Authorization: Bearer \(token)"],
    body: String = "{}", length: Int? = nil, version: String = "HTTP/1.1"
) -> Data {
    var lines = ["\(method) \(target) \(version)", "Host: 127.0.0.1"] + headers
    if let length {
        lines.append("Content-Length: \(length)")
    } else {
        lines.append("Content-Length: \(body.utf8.count)")
    }
    return Data((lines.joined(separator: "\r\n") + "\r\n\r\n" + body).utf8)
}

private func refused(_ data: Data) -> NativeHTTPError? {
    if case .refused(let error) = NativeHTTP.parse(data) { return error }
    return nil
}

@Test func parsesOnePost() throws {
    let data = request(body: #"{"path":"/a.pdf"}"#) + Data("GET / HTTP/1.1\r\n\r\n".utf8)
    guard case .request(let parsed) = NativeHTTP.parse(data) else {
        Issue.record("not parsed")
        return
    }
    #expect(parsed.method == "POST")
    #expect(parsed.target == NativeRoute.extractText.rawValue)
    #expect(parsed.authorization == "Bearer \(token)")
    #expect(parsed.body == Data(#"{"path":"/a.pdf"}"#.utf8))
}

@Test func waitsForTheRest() {
    let whole = request(body: "{\"path\":\"/a\"}")
    #expect(NativeHTTP.parse(whole.prefix(20)) == .incomplete)
    #expect(NativeHTTP.parse(whole.dropLast(3)) == .incomplete)
}

@Test func refusesWhatItDoesNotSpeak() {
    #expect(refused(request(headers: ["Transfer-Encoding: chunked"])) == .malformed)
    #expect(refused(request(version: "HTTP/1.0")) == .malformed)
    #expect(refused(request(headers: ["Content-Length: 2"])) == .malformed)
    #expect(refused(request(headers: ["Authorization: a", "Authorization: b"])) == .malformed)
    #expect(refused(request(headers: ["X-Folded: a", " continued"])) == .malformed)
    #expect(refused(request(headers: ["Bad Name: a"])) == .malformed)
    #expect(refused(request(headers: ["Content-Encoding: gzip"])) == .notJSON)
    #expect(refused(request(length: -1)) == .malformed)
    #expect(refused(request(length: 12_345_678)) == .malformed)
    #expect(refused(request(headers: ["X-Note: caf\u{E9}"])) == .malformed)
    let noLength = Data("POST / HTTP/1.1\r\nHost: x\r\n\r\n".utf8)
    #expect(refused(noLength) == .malformed)
}

@Test func refusesWhatIsTooLarge() {
    #expect(refused(request(length: NativeHTTP.maxBody + 1)) == .bodyTooLarge)
    let longHead = request(headers: ["X-Pad: " + String(repeating: "a", count: NativeHTTP.maxHead)])
    #expect(refused(longHead) == .headTooLarge)
    #expect(refused(Data(repeating: 0x61, count: NativeHTTP.maxHead)) == .headTooLarge)
    let many = (0...NativeHTTP.maxHeaders).map { "X-\($0): a" }
    #expect(refused(request(headers: many)) == .malformed)
}

private func route(_ data: Data) -> Result<NativeRoute, NativeHTTPError> {
    guard case .request(let parsed) = NativeHTTP.parse(data) else { return .failure(.malformed) }
    return NativeHTTP.route(parsed, token: token)
}

@Test func checksTheTokenFirst() {
    #expect(route(request()) == .success(.extractText))
    #expect(route(request("POST", NativeRoute.ping.rawValue)) == .success(.ping))
    let wrong = ["Content-Type: application/json", "Authorization: Bearer nope"]
    #expect(route(request("GET", "/x", headers: wrong)) == .failure(.unauthenticated))
    #expect(
        route(request(headers: ["Content-Type: application/json"])) == .failure(.unauthenticated))
    #expect(route(request("GET")) == .failure(.notPost))
    #expect(route(request("POST", "/hearth.v1.NativeService/Other")) == .failure(.notFound))
    let text = ["Content-Type: text/plain", "Authorization: Bearer \(token)"]
    #expect(route(request(headers: text)) == .failure(.notJSON))
    let charset = [
        "Content-Type: application/json; charset=utf-8", "Authorization: Bearer \(token)",
    ]
    #expect(route(request(headers: charset)) == .success(.extractText))
}

@Test func comparesTheWholeToken() {
    #expect(NativeHTTP.authorized("Bearer \(token)", token: token))
    #expect(!NativeHTTP.authorized("Bearer \(token)x", token: token))
    #expect(!NativeHTTP.authorized("Bearer \(token.dropLast())", token: token))
    #expect(!NativeHTTP.authorized("bearer \(token)", token: token))
    #expect(!NativeHTTP.authorized("Bearer ", token: ""))
    #expect(!NativeHTTP.authorized(nil, token: token))
    #expect(!NativeHTTP.authorized("", token: token))
}

@Test func repliesInConnectsShape() throws {
    let reply = String(bytes: NativeHTTP.reply(.unauthenticated), encoding: .utf8) ?? ""
    #expect(reply.hasPrefix("HTTP/1.1 401 Unauthorized\r\n"))
    #expect(reply.contains("Connection: close\r\n"))
    let body = try #require(reply.components(separatedBy: "\r\n\r\n").last)
    let json = try JSONSerialization.jsonObject(with: Data(body.utf8)) as? [String: String]
    #expect(json == ["code": "unauthenticated", "message": "the launch token does not match"])
}
