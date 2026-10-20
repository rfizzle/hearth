// swift-tools-version: 6.2
// Hearth: the app target (scenes, windows, helper lifecycle, wiring), the
// gallery target over the five local packages, and HearthReader, the
// sandboxed XPC service the app hands files to for text recognition. Imports
// point down: Hearth -> HearthFeatures -> HearthUI -> HearthDesign, HearthKit
// below all but HearthDesign, HearthNative only from Hearth, and HearthReader
// on HearthNative's HearthReading alone.
import PackageDescription

let package = Package(
    name: "Hearth",
    platforms: [.macOS(.v26)],
    products: [
        .executable(name: "Hearth", targets: ["Hearth"]),
        .executable(name: "HearthGallery", targets: ["HearthGallery"]),
        .executable(name: "HearthReader", targets: ["HearthReader"]),
    ],
    dependencies: [
        .package(path: "Packages/HearthDesign"),
        .package(path: "Packages/HearthUI"),
        .package(path: "Packages/HearthKit"),
        .package(path: "Packages/HearthFeatures"),
        .package(path: "Packages/HearthNative"),
    ],
    targets: [
        .executableTarget(
            name: "Hearth",
            dependencies: [
                .product(name: "HearthDesign", package: "HearthDesign"),
                .product(name: "HearthFeatures", package: "HearthFeatures"),
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "HearthNative", package: "HearthNative"),
                .product(name: "HearthUI", package: "HearthUI"),
            ],
            path: "Hearth/Sources"
        ),
        .testTarget(
            name: "HearthTests",
            dependencies: [
                "Hearth",
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "HearthNative", package: "HearthNative"),
            ],
            path: "Hearth/Tests/HearthTests"
        ),
        .executableTarget(
            name: "HearthGallery",
            dependencies: [
                .product(name: "HearthFeatures", package: "HearthFeatures"),
                .product(name: "HearthKit", package: "HearthKit"),
                .product(name: "HearthUI", package: "HearthUI"),
                .product(name: "HearthDesign", package: "HearthDesign"),
            ],
            path: "HearthGallery/Sources"
        ),
        .executableTarget(
            name: "HearthReader",
            dependencies: [.product(name: "HearthReading", package: "HearthNative")],
            path: "HearthReader/Sources"
        ),
    ],
    swiftLanguageModes: [.v6]
)
