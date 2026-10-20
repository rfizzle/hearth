// swift-tools-version: 6.2
// HearthUI: components and compositions that know hearth's models, drawn with
// HearthDesign's tokens. Features compose them; they decide nothing. The
// models they draw are HearthKit's generated types.
import PackageDescription

let package = Package(
    name: "HearthUI",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthUI", targets: ["HearthUI"])
    ],
    dependencies: [
        .package(path: "../HearthDesign"),
        .package(path: "../HearthKit"),
        // Markdown in a deliverable's text blocks only (D48); pinned exactly.
        .package(url: "https://github.com/gonzalezreal/swift-markdown-ui", exact: "2.4.1"),
    ],
    targets: [
        .target(
            name: "HearthUI",
            dependencies: [
                "HearthDesign",
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "MarkdownUI", package: "swift-markdown-ui"),
            ]
        ),
        .testTarget(name: "HearthUITests", dependencies: ["HearthUI"]),
    ],
    swiftLanguageModes: [.v6]
)
