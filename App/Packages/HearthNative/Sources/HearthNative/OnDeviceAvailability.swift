import FoundationModels

/// Whether the on-device model can answer right now. Callers that use the model
/// take one so a test can say "unavailable" without a Mac that lacks the model.
public struct OnDeviceAvailability: Sendable {
    /// Reports true only when the system says the model is available.
    public let isAvailable: @Sendable () -> Bool

    public init(isAvailable: @escaping @Sendable () -> Bool) {
        self.isAvailable = isAvailable
    }

    /// The system's answer. Not available covers every reason: no Apple
    /// Intelligence, model still downloading, unsupported device.
    public static let system = OnDeviceAvailability {
        if case .available = SystemLanguageModel.default.availability { return true }
        return false
    }

    /// A fixed answer, for tests.
    public static func fixed(_ value: Bool) -> OnDeviceAvailability {
        OnDeviceAvailability { value }
    }
}
