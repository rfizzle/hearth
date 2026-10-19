// swift-tools-version: 6.2
// HearthUI: components and compositions that know hearth's models, drawn with
// HearthDesign's tokens. Features compose them; they decide nothing.
import PackageDescription

let package = Package(
    name: "HearthUI",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthUI", targets: ["HearthUI"])
    ],
    dependencies: [
        .package(path: "../HearthDesign")
    ],
    targets: [
        .target(name: "HearthUI", dependencies: ["HearthDesign"]),
        .testTarget(name: "HearthUITests", dependencies: ["HearthUI"]),
    ],
    swiftLanguageModes: [.v6]
)
