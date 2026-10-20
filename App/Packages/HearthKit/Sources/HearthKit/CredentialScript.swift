// CredentialScript scripts a PreviewClient's credential calls for tests.

import Foundation
import Synchronization

/// What a PreviewClient answers the credential calls with, and what it was
/// asked: the records to store for each sync in turn (none once they run
/// out), the record each export returns, and the calls themselves, for a test
/// of the handshake. Values are whatever the test gives it, never real keys.
public final class CredentialScript: Sendable {
    private struct State {
        var toStore: [[Hearth_V1_Credential]]
        var exports: [String: Hearth_V1_Credential]
        var syncs: [[Hearth_V1_Credential]] = []
        var exported: [String] = []
    }
    private let state: Mutex<State>

    public init(
        toStore: [[Hearth_V1_Credential]] = [],
        exports: [String: Hearth_V1_Credential] = [:]
    ) {
        state = Mutex(State(toStore: toStore, exports: exports))
    }

    /// The records each sync was called with, in order.
    public var syncs: [[Hearth_V1_Credential]] { state.withLock { $0.syncs } }
    /// The names each export was called with, in order.
    public var exported: [String] { state.withLock { $0.exported } }

    func sync(_ records: [Hearth_V1_Credential]) -> [Hearth_V1_Credential] {
        state.withLock { state in
            state.syncs.append(records)
            return state.toStore.isEmpty ? [] : state.toStore.removeFirst()
        }
    }

    func export(_ name: String) -> Hearth_V1_Credential {
        state.withLock { state in
            state.exported.append(name)
            return state.exports[name] ?? .with { $0.name = name }
        }
    }
}
