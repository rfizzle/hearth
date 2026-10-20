// swift-tools-version: 6.2
// HearthNative: the adapters to the system (EventKit, Contacts, Shortcuts,
// Keychain, notifications, the on-device model, the global shortcut) and the
// server the core calls them through. Only the Hearth app target imports it.
// HearthKit gives the server the generated messages, and swift-protobuf their
// JSON.
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
        .package(path: "../HearthKit"),
        .package(url: "https://github.com/apple/swift-protobuf", from: "1.39.0"),
        .package(url: "https://github.com/sindresorhus/KeyboardShortcuts", exact: "3.1.0"),
    ],
    targets: [
        .target(
            name: "HearthNative",
            dependencies: [
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "KeyboardShortcuts", package: "KeyboardShortcuts"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ]
        ),
        .executableTarget(name: "OnDeviceSpike", dependencies: ["HearthNative"]),
        .testTarget(
            name: "HearthNativeTests",
            dependencies: ["HearthNative", .product(name: "HearthKit", package: "HearthKit")]
        ),
    ],
    swiftLanguageModes: [.v6]
)
