// swift-tools-version: 6.2
// HearthDesign: tokens by role and the primitives built on them. It imports
// nothing of hearth's; everything else in the app sits above it.
import PackageDescription

let package = Package(
    name: "HearthDesign",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthDesign", targets: ["HearthDesign"])
    ],
    targets: [
        .target(name: "HearthDesign", resources: [.process("Resources")]),
        .testTarget(name: "HearthDesignTests", dependencies: ["HearthDesign"]),
    ],
    swiftLanguageModes: [.v6]
)
