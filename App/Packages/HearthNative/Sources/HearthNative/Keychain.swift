// Keychain holds the owner's credentials the way every other Mac app's are
// held: one generic password item per record in the data protection keychain,
// readable only while the Mac is unlocked and never copied to another device.
// The app owns the records and pushes them to the core; the core keeps them in
// memory. Nothing here logs, and no error carries a value: an error names the
// step that failed and the OSStatus, never the item's data.
// See docs/decisions.md (D29), docs/architecture.md#8-third-party-packages

import Foundation
import Security
import os
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
///
/// The data protection keychain needs the app to be signed with a keychain
/// access group; an ad-hoc build, which is the one the owner runs from a
/// developer Mac, is refused with errSecMissingEntitlement (-34018). So the
/// first call probes once, and a refused probe moves every later call to the
/// login keychain, with the same service, account and accessibility. The
/// choice is made once per process.
public final class Keychain: KeychainStore {
    private let service: String
    private let choice = Mutex<Bool?>(nil)
    private let log = Logger(subsystem: "com.rfizzle.hearth", category: "keychain")

    /// Items live under `service`, the app's bundle id unless a test gives its
    /// own so it never touches the owner's records.
    public init(service: String = Bundle.main.bundleIdentifier ?? "com.rfizzle.hearth") {
        self.service = service
    }

    /// Whether items are in the data protection keychain (true) or the login
    /// keychain (false). Probes on first use; throws only when the probe fails
    /// for a reason other than the missing entitlement.
    public var usesDataProtection: Bool {
        get throws {
            try choice.withLock { chosen in
                if let chosen { return chosen }
                // A read answers "not found" without checking the entitlement, so
                // the probe writes an empty item and removes it at once.
                var query = baseAttributes()
                query[kSecUseDataProtectionKeychain as String] = true
                query[kSecAttrAccount as String] = "hearth-probe"
                query[kSecAttrAccessible as String] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
                var status = SecItemAdd(query as CFDictionary, nil)
                if status == errSecSuccess || status == errSecDuplicateItem {
                    query[kSecAttrAccessible as String] = nil
                    SecItemDelete(query as CFDictionary)
                    status = errSecSuccess
                }
                let protected: Bool
                switch status {
                case errSecSuccess:
                    protected = true
                case errSecMissingEntitlement:
                    protected = false
                default:
                    throw KeychainError(action: "reach the Keychain", status: status)
                }
                chosen = protected
                log.info(
                    "keychain store chosen: \(protected ? "data protection" : "login", privacy: .public)"
                )
                return protected
            }
        }
    }

    public func list() throws -> [String] {
        var query = try base()
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
        var query = try base(name)
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
        var status = SecItemUpdate(try base(name) as CFDictionary, attributes as CFDictionary)
        if status == errSecItemNotFound {
            status = SecItemAdd(try base(name).merging(attributes) { $1 } as CFDictionary, nil)
        }
        guard status == errSecSuccess else {
            throw KeychainError(action: "save a key", status: status)
        }
    }

    public func delete(_ name: String) throws {
        let status = SecItemDelete(try base(name) as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw KeychainError(action: "remove a saved key", status: status)
        }
    }

    /// The attributes every query carries; `name` narrows it to one record.
    private func base(_ name: String? = nil) throws -> [String: Any] {
        var query = baseAttributes()
        if try usesDataProtection { query[kSecUseDataProtectionKeychain as String] = true }
        if let name { query[kSecAttrAccount as String] = name }
        return query
    }

    private func baseAttributes() -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
        ]
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
