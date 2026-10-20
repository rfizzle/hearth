// swift-tools-version: 6.2
// HearthKit: the generated Connect client, the HearthClient protocol, the live
// client that speaks to hearthd and the preview client that serves fixtures,
// and the shaped deliverable's model with the shipped templates as resources.
// It imports no SwiftUI; everything above HearthDesign may import it.
import PackageDescription

let package = Package(
    name: "HearthKit",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthKit", targets: ["HearthKit"])
    ],
    dependencies: [
        .package(url: "https://github.com/connectrpc/connect-swift", from: "1.2.3"),
        .package(url: "https://github.com/apple/swift-protobuf", from: "1.39.0"),
    ],
    targets: [
        .target(
            name: "HearthKit",
            dependencies: [
                .product(name: "Connect", package: "connect-swift"),
                .product(name: "SwiftProtobuf", package: "swift-protobuf"),
            ],
            resources: [.copy("Resources/Deliverables")]
        ),
        .testTarget(name: "HearthKitTests", dependencies: ["HearthKit"]),
    ],
    swiftLanguageModes: [.v6]
)
