// Keychain holds the owner's credentials the way every other Mac app's are
// held: one generic password item per record in the data protection keychain,
// readable only while the Mac is unlocked and never copied to another device.
// The app owns the records and pushes them to the core; the core keeps them in
// memory. Nothing here logs, and no error carries a value: an error names the
// step that failed and the OSStatus, never the item's data.
// See docs/decisions.md (D29), docs/architecture.md#8-third-party-packages

import Foundation
import Security
import Synchronization

/// Where credential records are kept: the live Keychain, or a fake in tests.
public protocol KeychainStore: Sendable {
    /// The names of every record held.
    func list() throws -> [String]
    /// The record `name`, or nil when there is none.
    func load(_ name: String) throws -> Data?
    /// Stores `data` as `name`, replacing any record already there.
    func save(_ name: String, _ data: Data) throws
    /// Removes `name`; a record that is not there is not an error.
    func delete(_ name: String) throws
}

/// A Keychain call that failed, in words an owner can read.
public struct KeychainError: Error, LocalizedError, Equatable {
    /// What was being done, as a phrase: "read a saved key".
    public let action: String
    public let status: OSStatus

    public init(action: String, status: OSStatus) {
        self.action = action
        self.status = status
    }

    public var errorDescription: String? {
        "Hearth could not \(action) in the Keychain (error \(status))."
    }
}

/// The real Keychain, over the Security framework.
public struct Keychain: KeychainStore {
    private let service: String
    private let dataProtection: Bool

    /// Items live under `service`, the app's bundle id unless a test gives its
    /// own so it never touches the owner's records. The data protection
    /// keychain is the default; it needs the app to be signed with a keychain
    /// access group, so a process without one (a test, an ad-hoc build) is
    /// refused with errSecMissingEntitlement (-34018). Only a test turns it
    /// off, to exercise the calls themselves in the file keychain.
    public init(
        service: String = Bundle.main.bundleIdentifier ?? "com.rfizzle.hearth",
        dataProtection: Bool = true
    ) {
        self.service = service
        self.dataProtection = dataProtection
    }

    public func list() throws -> [String] {
        var query = base()
        query[kSecMatchLimit as String] = kSecMatchLimitAll
        query[kSecReturnAttributes as String] = true
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return [] }
        guard status == errSecSuccess else {
            throw KeychainError(action: "list the saved keys", status: status)
        }
        let items = result as? [[String: Any]] ?? []
        return items.compactMap { $0[kSecAttrAccount as String] as? String }.sorted()
    }

    public func load(_ name: String) throws -> Data? {
        var query = base(name)
        query[kSecMatchLimit as String] = kSecMatchLimitOne
        query[kSecReturnData as String] = true
        var result: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else {
            throw KeychainError(action: "read a saved key", status: status)
        }
        return result as? Data
    }

    public func save(_ name: String, _ data: Data) throws {
        let attributes: [String: Any] = [
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly,
        ]
        var status = SecItemUpdate(base(name) as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            status = SecItemAdd(base(name).merging(attributes) { $1 } as CFDictionary, nil)
        }
        guard status == errSecSuccess else {
            throw KeychainError(action: "save a key", status: status)
        }
    }

    public func delete(_ name: String) throws {
        let status = SecItemDelete(base(name) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError(action: "remove a saved key", status: status)
        }
    }

    /// The attributes every query carries; `name` narrows it to one record.
    private func base(_ name: String? = nil) -> [String: Any] {
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
        ]
        if dataProtection { query[kSecUseDataProtectionKeychain as String] = true }
        if let name { query[kSecAttrAccount as String] = name }
        return query
    }
}

/// A Keychain in memory, for tests of whatever uses one. `failure`, when set,
/// is thrown by every call so a test can see a failure handled.
public final class FakeKeychain: KeychainStore {
    private struct State {
        var records: [String: Data] = [:]
        var failure: KeychainError?
    }
    private let state = Mutex(State())

    public init(records: [String: Data] = [:]) {
        state.withLock { $0.records = records }
    }

    public var failure: KeychainError? {
        get { state.withLock { $0.failure } }
        set { state.withLock { $0.failure = newValue } }
    }

    /// Everything held, by name.
    public var records: [String: Data] {
        state.withLock { $0.records }
    }

    public func list() throws -> [String] {
        try state.withLock { state in
            if let failure = state.failure { throw failure }
            return state.records.keys.sorted()
        }
    }

    public func load(_ name: String) throws -> Data? {
        try state.withLock { state in
            if let failure = state.failure { throw failure }
            return state.records[name]
        }
    }

    public func save(_ name: String, _ data: Data) throws {
        try state.withLock { state in
            if let failure = state.failure { throw failure }
            state.records[name] = data
        }
    }

    public func delete(_ name: String) throws {
        try state.withLock { state in
            if let failure = state.failure { throw failure }
            state.records[name] = nil
        }
    }
}
