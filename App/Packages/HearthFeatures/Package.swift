// swift-tools-version: 6.2
// HearthFeatures: one folder per feature, each an @Observable store and the
// screens it drives. Stores cache and animate what the core sends; they decide
// nothing.
import PackageDescription

let package = Package(
    name: "HearthFeatures",
    platforms: [.macOS(.v26)],
    products: [
        .library(name: "HearthFeatures", targets: ["HearthFeatures"])
    ],
    dependencies: [
        .package(path: "../HearthDesign"),
        .package(path: "../HearthUI"),
        .package(path: "../HearthKit"),
    ],
    targets: [
        .target(
            name: "HearthFeatures",
            dependencies: [
                .product(name: "HearthDesign", package: "HearthDesign"),
                .product(name: "HearthUI", package: "HearthUI"),
                .product(name: "HearthKit", package: "HearthKit"),
            ]
        ),
        .testTarget(
            name: "HearthFeaturesTests",
            dependencies: [
                "HearthFeatures",
                .product(name: "HearthKit", package: "HearthKit"),
            ]
        ),
    ],
    swiftLanguageModes: [.v6]
)
