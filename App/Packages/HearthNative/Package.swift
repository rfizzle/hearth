// swift-tools-version: 6.2
// HearthNative: the adapters to the system (EventKit, Contacts, Shortcuts,
// Keychain, notifications, the on-device model, the global shortcut) and the
// server the core calls them through. Only the Hearth app target imports it.
// KeyboardShortcuts is pinned to an exact version: it registers system-wide
// hot keys, so a newer one is taken on purpose, not by resolution.
import PackageDescription

let package = Package(
    name: "HearthNative",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthNative", targets: ["HearthNative"])
    ],
    dependencies: [
        .package(url: "https://github.com/sindresorhus/KeyboardShortcuts", exact: "3.1.0")
    ],
    targets: [
        .target(
            name: "HearthNative",
            dependencies: [
                .product(name: "KeyboardShortcuts", package: "KeyboardShortcuts")
            ]
        ),
        .executableTarget(name: "OnDeviceSpike", dependencies: ["HearthNative"]),
        .testTarget(name: "HearthNativeTests", dependencies: ["HearthNative"]),
    ],
    swiftLanguageModes: [.v6]
)
