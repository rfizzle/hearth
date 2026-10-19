// LiveClient speaks the hearth.v1 contract to hearthd over loopback, with the
// per-launch token on every request. The token rides in the URLSession
// configuration's additional headers, so no interceptor is needed.

import Connect
import Foundation

/// The client that talks to a running hearthd.
public final class LiveClient: HearthClient {
    private let service: Hearth_V1_HearthServiceClient

    /// Connects to hearthd at `baseURL` (for example `http://127.0.0.1:52011`),
    /// sending `token` as a bearer token on every request.
    public init(baseURL: URL, token: String) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpAdditionalHeaders = ["Authorization": "Bearer \(token)"]
        let protocolClient = ProtocolClient(
            httpClient: URLSessionHTTPClient(configuration: configuration),
            config: ProtocolClientConfig(
                host: baseURL.absoluteString,
                networkProtocol: .connect,
                codec: ProtoCodec()
            )
        )
        self.service = Hearth_V1_HearthServiceClient(client: protocolClient)
    }

    public func ping() async throws -> String {
        let response = await service.ping(request: Hearth_V1_PingRequest())
        switch response.result {
        case .success(let message):
            return message.version
        case .failure(let error):
            throw error
        }
    }

    public func streamHello(count: Int32) -> AsyncThrowingStream<String, Error> {
        let stream = service.streamHello()
        return AsyncThrowingStream { continuation in
            let task = Task {
                do {
                    var request = Hearth_V1_HelloRequest()
                    request.count = count
                    try stream.send(request)
                } catch {
                    continuation.finish(throwing: error)
                    return
                }
                for await result in stream.results() {
                    switch result {
                    case .headers:
                        continue
                    case .message(let chunk):
                        continuation.yield(chunk.word)
                    case .complete(_, let error, _):
                        continuation.finish(throwing: error)
                        return
                    }
                }
                continuation.finish()
            }
            continuation.onTermination = { _ in
                task.cancel()
                stream.cancel()
            }
        }
    }
}
