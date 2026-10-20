// CredentialSync keeps the Keychain and the core's memory in step. The app owns
// the records: for each helper it reads every one, hands them to the core, stores
// whatever the core still held in files, and hands them over again so the core
// can drop those files. After that the core announces each change by name and
// the app asks for that record and saves it, or deletes it. One queue runs
// the handshake and the changes in order, so a change announced while the
// handshake runs waits for it.
//
// Values live only in the Credential messages passed between the Keychain and
// the client in these functions. Nothing here logs, and a failure reaches the
// status line as a sentence that names the step, never a name or a value.
// See docs/decisions.md (D29), docs/architecture.md#2-responsibilities

import HearthKit
import HearthNative

@MainActor
final class CredentialSync {
    private let keychain: any KeychainStore
    private let report: @MainActor (String) -> Void
    private var client: (any HearthClient)?
    private var tail: Task<Void, Never>?

    /// A sync over `keychain`; `report` takes the sentence for the status line
    /// when a step fails.
    init(keychain: any KeychainStore, report: @escaping @MainActor (String) -> Void) {
        self.keychain = keychain
        self.report = report
    }

    /// A new helper is up on `client`: runs the handshake after anything already
    /// queued.
    func helperStarted(client: any HearthClient) {
        self.client = client
        enqueue { await self.handshake(with: client) }
    }

    /// The helper has gone; changes announced from now on are not for it.
    func helperGone() {
        client = nil
    }

    /// The relay's handler: applies one `credential_changed` event, ignoring
    /// every other kind.
    func handle(_ event: Hearth_V1_SubscribeResponse) async {
        guard case .credentialChanged(let change) = event.event, let client else { return }
        await enqueue { await self.apply(change, from: client) }.value
    }

    /// Returns when everything queued so far has finished.
    func settle() async {
        await tail?.value
    }

    @discardableResult
    private func enqueue(_ work: @escaping @MainActor () async -> Void) -> Task<Void, Never> {
        let previous = tail
        let task = Task { @MainActor in
            await previous?.value
            await work()
        }
        tail = task
        return task
    }

    /// Reads every record, syncs, stores what the core returns, and syncs again.
    private func handshake(with client: any HearthClient) async {
        do {
            let returned = try await client.syncCredentials(try records())
            for record in returned where !record.name.isEmpty && !record.value.isEmpty {
                try keychain.save(record.name, record.value)
            }
            _ = try await client.syncCredentials(try records())
        } catch {
            fail("sync the saved keys with the helper", error)
        }
    }

    /// Saves the record the core announced, or removes it when it was deleted.
    private func apply(_ change: Hearth_V1_CredentialChanged, from client: any HearthClient) async {
        guard !change.name.isEmpty else { return }
        do {
            if change.deleted {
                try keychain.delete(change.name)
            } else {
                let record = try await client.exportCredential(name: change.name)
                if !record.value.isEmpty { try keychain.save(change.name, record.value) }
            }
        } catch {
            fail("keep a changed key", error)
        }
    }

    /// Every record in the Keychain that holds a value; the core refuses an empty
    /// one, so an empty item is left out rather than failing every handshake.
    private func records() throws -> [Hearth_V1_Credential] {
        try keychain.list().compactMap { name in
            guard let value = try keychain.load(name), !value.isEmpty else { return nil }
            return .with {
                $0.name = name
                $0.value = value
            }
        }
    }

    /// Tells the status line which step failed. A Keychain error carries its own
    /// sentence (the step and the OSStatus); any other error is the helper's, and
    /// its text is left out.
    private func fail(_ step: String, _ error: any Error) {
        if let error = error as? KeychainError {
            report(error.localizedDescription)
        } else {
            report("Hearth could not \(step).")
        }
    }
}
