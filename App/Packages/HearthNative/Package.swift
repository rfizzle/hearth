// swift-tools-version: 6.2
// HearthNative: the adapters to the system (EventKit, Contacts, Shortcuts,
// Keychain, notifications, the on-device model) and the server the core calls
// them through. Only the Hearth app target imports it.
import PackageDescription

let package = Package(
    name: "HearthNative",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthNative", targets: ["HearthNative"])
    ],
    targets: [
        .target(name: "HearthNative"),
        .executableTarget(name: "OnDeviceSpike", dependencies: ["HearthNative"]),
        .testTarget(name: "HearthNativeTests", dependencies: ["HearthNative"]),
    ],
    swiftLanguageModes: [.v6]
)
